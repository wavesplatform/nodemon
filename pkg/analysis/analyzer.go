package analysis

import (
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type Analyzer struct {
	es     *events.Storage
	socket protocol.Socket
}

func NewAnalyzer(es *events.Storage, socket protocol.Socket) *Analyzer {
	return &Analyzer{es: es, socket: socket}
}

func (a *Analyzer) analyze(alerts chan<- entities.Alert, pollingResult *entities.OnPollingComplete) error {
	// TODO: analysis here
	nodes := make(entities.NodeStatements, 0, len(pollingResult.Nodes()))
	err := a.es.ViewStatementsByTimestamp(pollingResult.Timestamp(), func(statement *entities.NodeStatement) bool {
		nodes = append(nodes, *statement)
		return true
	})
	if err != nil {
		return errors.Wrap(err, "failed to analyze nodes statements")
	}
	statusSplit := nodes.Iterator().SplitByNodeStatus()

	for _, unreachable := range statusSplit[entities.Unreachable] {
		alerts <- entities.Alert{Description: fmt.Sprintf(
			"[%s] Node %q is UNREACHABLE.",
			time.Unix(unreachable.Timestamp, 0).String(), unreachable.Node,
		)}
	}

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
