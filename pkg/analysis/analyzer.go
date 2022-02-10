package analysis

import (
	"log"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing"
)

type Analyzer struct {
	es *storing.EventsStorage
}

func NewAnalyzer(es *storing.EventsStorage) *Analyzer {
	return &Analyzer{es: es}
}

func (a *Analyzer) analyze(alerts chan<- Alert, pollingResult *entities.OnPollingComplete) error {
	// TODO: analysis here
	return nil
}

func (a *Analyzer) Start(notifications <-chan entities.Notification) <-chan Alert {
	out := make(chan Alert)
	go func(alerts chan<- Alert) {
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
		close(alerts)
	}(out)
	return out
}
