package analysis

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type AnalyzerOptions struct {
	AlertBackoff            int
	AlertVacuumQuota        int
	AlertConfirmations      map[entities.AlertType]int
	UnreachableCriteriaOpts *criteria.UnreachableCriterionOptions
	IncompleteCriteriaOpts  *criteria.IncompleteCriterionOptions
	HeightCriteriaOpts      *criteria.HeightCriterionOptions
	StateHashCriteriaOpts   *criteria.StateHashCriterionOptions
	BaseTargetCriterionOpts *criteria.BaseTargetCriterionOptions
}

type Analyzer struct {
	es   *events.Storage
	as   *alertsStorage
	opts *AnalyzerOptions
	zap  *zap.Logger
}

func NewAnalyzer(es *events.Storage, opts *AnalyzerOptions, logger *zap.Logger) *Analyzer {
	if opts == nil {
		opts = &AnalyzerOptions{}
	}
	if opts.AlertBackoff == 0 {
		opts.AlertBackoff = defaultAlertBackoff
	}
	if opts.AlertVacuumQuota == 0 {
		opts.AlertVacuumQuota = defaultAlertVacuumQuota
	}
	if opts.AlertConfirmations == nil {
		opts.AlertConfirmations = defaultAlertConfirmations()
	}

	as := newAlertsStorage(opts.AlertBackoff, opts.AlertVacuumQuota, opts.AlertConfirmations, logger)
	return &Analyzer{es: es, as: as, opts: opts, zap: logger}
}

func (a *Analyzer) analyze(alerts chan<- entities.Alert, pollingResult entities.NodesGatheringNotification) error {
	statements := make(entities.NodeStatements, 0, pollingResult.NodesCount())
	err := a.es.ViewStatementsByTimestamp(pollingResult.Timestamp(), func(statement *entities.NodeStatement) bool {
		statements = append(statements, *statement)
		return true
	})
	if err != nil {
		return errors.Wrap(err, "failed to analyze nodes statements")
	}

	var (
		wg          = new(sync.WaitGroup)
		criteriaOut = make(chan entities.Alert)
		ctx, cancel = context.WithCancel(context.Background())
		proxyDone   = make(chan struct{})
	)
	defer func() {
		wg.Wait()   // wait for all criterion routines
		cancel()    // stop analyzer proxy
		<-proxyDone // wait for analyzer proxy
	}()

	// run criterion routines
	routines := a.criteriaRoutines(statements, pollingResult.Timestamp())
	wg.Add(len(routines))
	for _, f := range routines {
		go func(f func(in chan<- entities.Alert) error) {
			defer wg.Done()
			if err := f(criteriaOut); err != nil {
				a.zap.Error("Error occurred on criterion routine", zap.Error(err))
				criteriaOut <- entities.NewInternalErrorAlert(pollingResult.Timestamp(), err)
			}
		}(f)
	}
	// run analyzer proxy
	go func(ctx context.Context, alertsIn chan<- entities.Alert, criteriaOut <-chan entities.Alert) {
		defer close(proxyDone)
		defer func() {
			// we have to vacuum alerts storage each time and send alerts about fixed alerts :)
			// also we perform vacuum here to prevent data race condition
			ts := pollingResult.Timestamp()
			vacuumedAlerts := a.as.Vacuum()
			for _, alert := range vacuumedAlerts {
				alertsIn <- &entities.AlertFixed{
					Timestamp:      ts,
					Fixed:          alert,
					FixedAlertType: alert.Type(),
				}
			}
		}()
		for {
			select {
			case alert := <-criteriaOut:
				sendAlertNow := a.as.PutAlert(alert)
				if sendAlertNow {
					alertsIn <- alert
				}
			case <-ctx.Done():
				return
			}
		}
	}(ctx, alerts, criteriaOut)
	return nil
}

func (a *Analyzer) criteriaRoutines(statements entities.NodeStatements, timestamp int64) []func(in chan<- entities.Alert) error {
	statusSplit := statements.SplitByNodeStatus()
	for _, nodeStatements := range statusSplit {
		nodeStatements.SortByNodeAsc()
	}
	return []func(in chan<- entities.Alert) error{
		func(in chan<- entities.Alert) error {
			criterion := criteria.NewIncompleteCriterion(a.es, a.opts.IncompleteCriteriaOpts, a.zap)
			return criterion.Analyze(in, statusSplit[entities.Incomplete])
		},
		func(in chan<- entities.Alert) error {
			for _, statement := range statusSplit[entities.InvalidHeight] {
				in <- &entities.InvalidHeightAlert{NodeStatement: statement}
			}
			return nil
		},
		func(in chan<- entities.Alert) error {
			criterion := criteria.NewUnreachableCriterion(a.es, a.opts.UnreachableCriteriaOpts, a.zap)
			return criterion.Analyze(in, timestamp, statusSplit[entities.Unreachable])
		},
		func(in chan<- entities.Alert) error {
			criterion := criteria.NewHeightCriterion(a.opts.HeightCriteriaOpts, a.zap)
			criterion.Analyze(in, timestamp, statusSplit[entities.OK])
			return nil
		},
		func(in chan<- entities.Alert) error {
			criterion := criteria.NewStateHashCriterion(a.es, a.opts.StateHashCriteriaOpts, a.zap)
			return criterion.Analyze(in, timestamp, statusSplit[entities.OK])
		},
		func(in chan<- entities.Alert) error {
			criterion, err := criteria.NewBaseTargetCriterion(a.opts.BaseTargetCriterionOpts)
			if err != nil {
				return err
			}
			criterion.Analyze(in, timestamp, statusSplit[entities.OK])
			return nil
		},
	}
}

func (a *Analyzer) Start(notifications <-chan entities.NodesGatheringNotification) <-chan entities.Alert {
	out := make(chan entities.Alert)
	go func(alerts chan<- entities.Alert) {
		defer close(alerts)
		for n := range notifications {
			err := a.processNotification(alerts, n)
			if err != nil {
				a.zap.Error("Failed to process notification", zap.Error(err))
				ts := time.Now().Unix()
				alerts <- entities.NewInternalErrorAlert(ts, err)
			}
		}
	}(out)
	return out
}

func (a *Analyzer) processNotification(alerts chan<- entities.Alert, n entities.NodesGatheringNotification) error {
	a.zap.Sugar().Infof("Statements gathering completed with %d nodes", n.NodesCount())
	cnt, err := a.es.StatementsCount()
	if err != nil {
		return errors.Wrap(err, "failed to get statements count")
	}
	a.zap.Sugar().Infof("Total statements count: %d", cnt)
	if err := a.analyze(alerts, n); err != nil {
		return errors.Wrap(err, "failed to analyze nodes statements")
	}
	return nil
}
