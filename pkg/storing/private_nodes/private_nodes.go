package private_nodes

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type PrivateNodesEventsWriter interface {
	Write(event entities.EventWithTimestampProducer, url string)
}

type privateNodesEvents struct {
	mu   *sync.RWMutex
	data map[string]entities.EventWithTimestampProducer // map[url]NodeStatement
}

func newPrivateNodesEvents() *privateNodesEvents {
	return &privateNodesEvents{
		mu:   new(sync.RWMutex),
		data: make(map[string]entities.EventWithTimestampProducer),
	}
}

func (p *privateNodesEvents) Write(producer entities.EventWithTimestampProducer, url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data[url] = producer
}

type PrivateNodesHandler struct {
	es            *events.Storage
	zap           *zap.Logger
	privateEvents *privateNodesEvents
}

func NewPrivateNodesHandler(es *events.Storage, zap *zap.Logger) *PrivateNodesHandler {
	return &PrivateNodesHandler{
		es:            es,
		zap:           zap,
		privateEvents: newPrivateNodesEvents(),
	}
}

func (h *PrivateNodesHandler) PrivateNodesEventsWriter() PrivateNodesEventsWriter {
	return h.privateEvents
}

func (h *PrivateNodesHandler) putPrivateNodesEvents(ts int64) entities.Nodes {
	nodes := make(entities.Nodes, 0, len(h.privateEvents.data))
	h.privateEvents.mu.RLock()
	defer h.privateEvents.mu.RUnlock()
	for node, eventProducer := range h.privateEvents.data {
		event := eventProducer.WithTimestamp(ts)
		if err := h.handleEventWithTs(ts, event); err != nil {
			h.zap.Error("Failed to put private node event", zap.Error(err))
			return nodes
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (h *PrivateNodesHandler) handleEventWithTs(ts int64, e entities.Event) error {
	switch e := e.(type) {
	case *entities.InvalidHeightEvent:
		updatedEvent := entities.NewInvalidHeightEvent(e.Node(), ts, e.Version(), e.Height())
		err := h.es.PutEvent(updatedEvent)
		if err != nil {
			return errors.Wrapf(err, "failed to put event (%T) for node %s", e, e.Node())
		}
		h.zap.Sugar().Infof("Statement (%T) for private node %s has been put into the storage, height %d", e, e.Node(), e.Height())
	case *entities.StateHashEvent:
		updatedEvent := entities.NewStateHashEvent(e.Node(), ts, e.Version(), e.Height(), e.StateHash(), e.BaseTarget())
		err := h.es.PutEvent(updatedEvent)
		if err != nil {
			return errors.Wrapf(err, "failed to put event (%T) for node %s", e, e.Node())
		}
		h.zap.Sugar().Infof("Statement (%T) for private node %s has been put into the storage, height %d, statehash %s",
			e, e.Node(), e.Height(), e.StateHash().SumHash.Hex(),
		)
	default:
		return errors.Errorf("unknown event type (%T) for node %s", e, e.Node())
	}
	return nil
}

func (h *PrivateNodesHandler) Run(input <-chan entities.Notification) <-chan entities.Notification {
	output := make(chan entities.Notification)
	go h.handlePrivateEvents(input, output)
	return output
}

func (h *PrivateNodesHandler) handlePrivateEvents(input <-chan entities.Notification, output chan<- entities.Notification) {
	defer close(output)
	for wn := range input {
		switch notification := wn.(type) {
		case *entities.OnPollingComplete:
			var (
				ts          = notification.Timestamp()
				polledNodes = notification.Nodes()
			)
			storedPrivateNodes := h.putPrivateNodesEvents(ts)
			h.zap.Sugar().Infof("Total count of stored private nodes statements is %d at timestamp %d", len(storedPrivateNodes), ts)
			output <- entities.NewOnPollingComplete(append(polledNodes, storedPrivateNodes...), ts)
		default:
			h.zap.Error("unknown analyzer notification", zap.String("type", fmt.Sprintf("%T", notification)))
		}
	}
}
