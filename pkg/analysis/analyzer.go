package analysis

import (
	"context"
	"iter"
	"sync"
	"time"

	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/analysis/storage"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type AnalyzerOptions struct {
	AlertBackoff            int
	AlertVacuumQuota        int
	AlertConfirmations      []storage.AlertConfirmationsValue
	UnreachableCriteriaOpts *criteria.UnreachableCriterionOptions
	IncompleteCriteriaOpts  *criteria.IncompleteCriterionOptions
	HeightCriteriaOpts      *criteria.HeightCriterionOptions
	StateHashCriteriaOpts   *criteria.StateHashCriterionOptions
	BaseTargetCriterionOpts *criteria.BaseTargetCriterionOptions
	ChallengeCriterionOpts  *criteria.ChallengedBlockCriterionOptions
}

type Analyzer struct {
	es   *events.Storage
	as   *storage.AlertsStorage
	opts *AnalyzerOptions
	zap  *zap.Logger
}

const (
	heightAlertConfirmationsDefault = 2
)

func NewAnalyzer(es *events.Storage, opts *AnalyzerOptions, logger *zap.Logger) *Analyzer {
	if opts == nil {
		opts = &AnalyzerOptions{}
	}
	if opts.AlertBackoff == 0 {
		opts.AlertBackoff = storage.DefaultAlertBackoff
	}
	if opts.AlertVacuumQuota == 0 {
		opts.AlertVacuumQuota = storage.DefaultAlertVacuumQuota
	}
	if opts.AlertConfirmations == nil {
		opts.AlertConfirmations = []storage.AlertConfirmationsValue{
			{
				AlertType:     entities.HeightAlertType,
				Confirmations: heightAlertConfirmationsDefault,
			},
		}
	}

	as := storage.NewAlertsStorage(logger,
		storage.AlertBackoff(opts.AlertBackoff),
		storage.AlertVacuumQuota(opts.AlertVacuumQuota),
		storage.AlertConfirmations(opts.AlertConfirmations...),
	)
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
			if routineErr := f(criteriaOut); routineErr != nil {
				a.zap.Error("Error occurred on criterion routine", zap.Error(routineErr))
				criteriaOut <- entities.NewInternalErrorAlert(pollingResult.Timestamp(), routineErr)
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
					Timestamp: ts,
					Fixed:     alert,
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

func joinSlicesSeq2[Slice ~[]E, E any](slices ...Slice) iter.Seq2[int, E] {
	return func(yield func(int, E) bool) {
		var i int
		for _, slice := range slices {
			for _, v := range slice {
				if !yield(i, v) {
					return
				}
				i++
			}
		}
	}
}

func (a *Analyzer) criteriaRoutines(
	statements entities.NodeStatements,
	timestamp int64,
) []func(in chan<- entities.Alert) error {
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
			criterion := criteria.NewChallengedBlockCriterion(a.opts.ChallengeCriterionOpts, a.zap)
			statementsToAnalyze := joinSlicesSeq2(statusSplit[entities.Incomplete], statusSplit[entities.OK])
			criterion.Analyze(in, timestamp, statementsToAnalyze)
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
				ia := entities.NewInternalErrorAlert(ts, errors.Wrap(err, "analyzer: failed to process notification"))
				alerts <- ia
			}
		}
	}(out)
	return out
}

func (a *Analyzer) processNotification(alerts chan<- entities.Alert, n entities.NodesGatheringNotification) error {
	if err := n.Error(); err != nil {
		alerts <- entities.NewInternalErrorAlert(n.Timestamp(), err)
	}
	nodesCount := n.NodesCount()
	if nodesCount == 0 { // nothing to analyze
		return nil
	}
	a.zap.Sugar().Infof("Statements gathering completed with %d nodes", nodesCount)
	cnt, err := a.es.StatementsCount()
	if err != nil {
		return errors.Wrap(err, "failed to get statements count")
	}
	a.zap.Sugar().Infof("Total statements count: %d", cnt)
	if analyzeErr := a.analyze(alerts, n); analyzeErr != nil {
		return errors.Wrap(analyzeErr, "failed to analyze nodes statements")
	}
	return nil
}
