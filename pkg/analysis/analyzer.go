package analysis

import (
	"context"
	"log"
	"sync"

	"github.com/pkg/errors"
	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type AnalyzerOptions struct {
	AlertBackoff            int
	AlertVacuumQuota        int
	UnreachableCriteriaOpts *criteria.UnreachableCriterionOptions
	HeightCriteriaOpts      *criteria.HeightCriterionOptions
	StateHashCriteriaOpts   *criteria.StateHashCriterionOptions
}

type Analyzer struct {
	es   *events.Storage
	as   *alertsStorage
	opts *AnalyzerOptions
}

func NewAnalyzer(es *events.Storage, opts *AnalyzerOptions) *Analyzer {
	if opts == nil {
		opts = &AnalyzerOptions{}
	}
	if opts.AlertBackoff == 0 {
		opts.AlertBackoff = defaultAlertBackoff
	}
	if opts.AlertVacuumQuota == 0 {
		opts.AlertVacuumQuota = defaultAlertVacuumQuota
	}

	as := newAlertsStorage(opts.AlertBackoff, opts.AlertVacuumQuota)
	return &Analyzer{es: es, as: as, opts: opts}
}

func (a *Analyzer) analyze(alerts chan<- entities.Alert, pollingResult *entities.OnPollingComplete) error {
	statements := make(entities.NodeStatements, 0, len(pollingResult.Nodes()))
	err := a.es.ViewStatementsByTimestamp(pollingResult.Timestamp(), func(statement *entities.NodeStatement) bool {
		statements = append(statements, *statement)
		return true
	})
	if err != nil {
		return errors.Wrap(err, "failed to analyze nodes statements")
	}
	statusSplit := statements.SplitByNodeStatus()

	routines := [...]func(in chan<- entities.Alert) error{
		func(in chan<- entities.Alert) error {
			for _, statement := range statusSplit[entities.Incomplete] {
				in <- &entities.IncompleteAlert{NodeStatement: statement}
			}
			return nil
		},
		func(in chan<- entities.Alert) error {
			for _, statement := range statusSplit[entities.InvalidHeight] {
				in <- &entities.InvalidHeightAlert{NodeStatement: statement}
			}
			return nil
		},
		func(in chan<- entities.Alert) error {
			criterion := criteria.NewUnreachableCriterion(a.es, a.opts.UnreachableCriteriaOpts)
			return criterion.Analyze(in, pollingResult.Timestamp(), statusSplit[entities.Unreachable])
		},
		func(in chan<- entities.Alert) error {
			criterion := criteria.NewHeightCriterion(a.opts.HeightCriteriaOpts)
			criterion.Analyze(in, pollingResult.Timestamp(), statusSplit[entities.OK])
			return nil
		},
		func(in chan<- entities.Alert) error {
			criterion := criteria.NewStateHashCriterion(a.es, a.opts.StateHashCriteriaOpts)
			return criterion.Analyze(in, pollingResult.Timestamp(), statusSplit[entities.OK])
		},
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
	wg.Add(len(routines))
	for _, f := range routines {
		go func(f func(in chan<- entities.Alert) error) {
			defer wg.Done()
			if err := f(criteriaOut); err != nil {
				log.Printf("Error occured on criterion routine: %v", err)
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

func (a *Analyzer) Start(notifications <-chan entities.Notification) <-chan entities.Alert {
	out := make(chan entities.Alert)
	go func(alerts chan<- entities.Alert) {
		defer close(alerts)
		for n := range notifications {
			switch notificcationType := n.(type) {
			case *entities.OnPollingComplete:
				log.Printf("On polling complete of %d nodes", len(notificcationType.Nodes()))
				cnt, err := a.es.StatementsCount()
				if err != nil {
					log.Printf("Failed to query statements: %v", err)
				}
				log.Printf("Total statemetns count: %d", cnt)

				if err := a.analyze(alerts, notificcationType); err != nil {
					log.Printf("Failed to analyze nodes: %v", err)
				}
			default:
				log.Printf("Unknown alanyzer notification (%T)", notificcationType)
			}
		}
	}(out)
	return out
}
