package private_nodes

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type PrivateNodesEvents struct {
	Mu   *sync.RWMutex
	Data map[string]entities.Event // map[url]NodeStatement
}

func NewPrivateNodesEvents() *PrivateNodesEvents {
	return &PrivateNodesEvents{
		Mu:   new(sync.RWMutex),
		Data: make(map[string]entities.Event),
	}
}

func (p *PrivateNodesEvents) Write(event entities.Event, url string) {
	p.Mu.Lock()
	defer p.Mu.Unlock()
	p.Data[url] = event
}

func (p *PrivateNodesEvents) Read(url string) entities.Event {
	p.Mu.RLock()
	defer p.Mu.RUnlock()
	if _, ok := p.Data[url]; ok {
		return p.Data[url]
	}
	return nil
}

type PrivateNodesHandler struct {
	es            *events.Storage
	zap           *zap.Logger
	privateEvents *PrivateNodesEvents
}

func NewPrivateNodesHandler(es *events.Storage, zap *zap.Logger, privateNodesEvents *PrivateNodesEvents) *PrivateNodesHandler {
	return &PrivateNodesHandler{
		es:            es,
		zap:           zap,
		privateEvents: privateNodesEvents,
	}
}

func (h *PrivateNodesHandler) putPrivateNodesEvents(ts int64) entities.Nodes {
	nodes := make(entities.Nodes, 0, len(h.privateEvents.Data))
	h.privateEvents.Mu.RLock()
	defer h.privateEvents.Mu.RUnlock()
	for node, event := range h.privateEvents.Data {
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

func (h *PrivateNodesHandler) HandlePrivateEvents(wrappedNotifications <-chan entities.WrappedNotification, notifications chan<- entities.Notification) {
	for wn := range wrappedNotifications {
		switch notification := wn.(type) {
		case *entities.OnPollingComplete:
			var (
				ts          = notification.Ts
				polledNodes = notification.Nodes()
			)
			storedPrivateNodes := h.putPrivateNodesEvents(ts)
			h.zap.Sugar().Infof("Total count of stored private nodes statements is %d at timestamp %d", len(storedPrivateNodes), ts)
			notifications <- entities.NewOnPollingComplete(append(polledNodes, storedPrivateNodes...), ts)
		default:
			h.zap.Error("unknown analyzer notification", zap.String("type", fmt.Sprintf("%T", notification)))
		}
	}
}
