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

type AnalyzerCriteriaOptions struct {
	UnreachableOpts *criteria.UnreachableCriterionOptions
	HeightOpts      *criteria.HeightCriterionOptions
	StateHashOpts   *criteria.StateHashCriterionOptions
}

type Analyzer struct {
	es   *events.Storage
	opts *AnalyzerCriteriaOptions
}

func NewAnalyzer(es *events.Storage, opts *AnalyzerCriteriaOptions) *Analyzer {
	if opts == nil {
		opts = &AnalyzerCriteriaOptions{} // use default
	}
	return &Analyzer{es: es, opts: opts}
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
			// TODO(nickeskov): configure it
			criterion := criteria.NewUnreachableCriterion(a.es, a.opts.UnreachableOpts)
			return criterion.Analyze(in, pollingResult.Timestamp(), statusSplit[entities.Unreachable])
		},
		func(in chan<- entities.Alert) error {
			// TODO(nickeskov): configure it
			criterion := criteria.NewHeightCriterion(a.opts.HeightOpts)
			criterion.Analyze(in, pollingResult.Timestamp(), statusSplit[entities.OK])
			return nil
		},
		func(in chan<- entities.Alert) error {
			// TODO(nickeskov): configure it
			criterion := criteria.NewStateHashCriterion(a.es, a.opts.StateHashOpts)
			return criterion.Analyze(in, pollingResult.Timestamp(), statusSplit[entities.OK])
		},
	}
	var (
		wg          = new(sync.WaitGroup)
		criteriaOut = make(chan entities.Alert)
		ctx, cancel = context.WithCancel(context.Background())
	)
	defer func() {
		wg.Wait()
		cancel()
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
		for {
			select {
			case alert := <-criteriaOut:
				if err := a.sendAlert(alertsIn, alert); err != nil {
					log.Printf("Some error orrured on analyzer proxy routine: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}(ctx, alerts, criteriaOut)
	return nil
}

func (a *Analyzer) sendAlert(alerts chan<- entities.Alert, alert entities.Alert) error {
	// TODO(nickeskov): handle ignore rules
	alerts <- alert
	return nil
}

func (a *Analyzer) Start(notifications <-chan entities.Notification) <-chan entities.Alert {
	out := make(chan entities.Alert)
	go func(alerts chan<- entities.Alert) {
		defer close(alerts)
		for n := range notifications {
			switch tn := n.(type) {
			case *entities.OnPollingComplete:
				log.Printf("On polling complete of %d nodes", len(tn.Nodes()))
				cnt, err := a.es.StatementsCount()
				if err != nil {
					log.Printf("Failed to query statements: %v", err)
				}
				log.Printf("Total statemetns count: %d", cnt)

				if err := a.analyze(alerts, tn); err != nil {
					log.Printf("Failed to analyze nodes: %v", err)
				}
			default:
				log.Printf("Unknown alanyzer notification (%T)", tn)
			}
		}
	}(out)
	return out
}
