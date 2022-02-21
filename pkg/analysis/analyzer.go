package analysis

import (
	"go.nanomsg.org/mangos/v3/protocol"
	"log"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing"
)

type Analyzer struct {
	es     *storing.EventsStorage
	socket protocol.Socket
}

func NewAnalyzer(es *storing.EventsStorage, socket protocol.Socket) *Analyzer {
	return &Analyzer{es: es, socket: socket}
}

func (a *Analyzer) analyze(alerts chan<- entities.Alert, pollingResult *entities.OnPollingComplete) error {
	// TODO: analysis here
	return nil
}

func (a *Analyzer) Start(notifications <-chan entities.Notification) <-chan entities.Alert {
	out := make(chan entities.Alert)
	go func(alerts chan<- entities.Alert) {
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
