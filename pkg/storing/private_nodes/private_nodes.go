package private_nodes

import (
	"fmt"
	"sync"

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
		switch e := event.(type) {
		case *entities.InvalidHeightEvent:
			updatedEvent := entities.NewInvalidHeightEvent(e.Node(), ts, e.Version(), e.Height())
			err := h.es.PutEvent(updatedEvent)
			if err != nil {
				h.zap.Error("Failed to put event", zap.Error(err))
				return nodes
			}
			h.zap.Sugar().Infof("Statement 'InvalidHeight' for private node %s has been put into the storage, height %d", node, e.Height())
		case *entities.StateHashEvent:
			updatedEvent := entities.NewStateHashEvent(e.Node(), ts, e.Version(), e.Height(), e.StateHash(), e.BaseTarget())
			err := h.es.PutEvent(updatedEvent)
			if err != nil {
				h.zap.Error("Failed to put event", zap.Error(err))
				return nodes
			}
			h.zap.Sugar().Infof("Statement for private node %s has been put into the storage, height %d, statehash %s", node, e.Height(), e.StateHash().SumHash.Hex())
		default:
			h.zap.Sugar().Errorf("Unknown event type (%T)", e)
			return nodes
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (h *PrivateNodesHandler) HandlePrivateEvents(wrappedNotifications <-chan entities.WrappedNotification, notifications chan<- entities.Notification) {
	for wn := range wrappedNotifications {
		switch notification := wn.(type) {
		case *entities.OnPollingComplete:
			polledNodes := notification.Nodes()
			storedPrivateNodes := h.putPrivateNodesEvents(notification.Ts)
			notifications <- entities.NewOnPollingComplete(append(polledNodes, storedPrivateNodes...), notification.Ts)
		default:
			h.zap.Error("unknown analyzer notification", zap.String("type", fmt.Sprintf("%T", notification)))
		}
	}
}
