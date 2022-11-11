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

func (h *PrivateNodesHandler) putPrivateNodesEvents(ts int64) {

	h.privateEvents.Mu.RLock()
	defer h.privateEvents.Mu.RUnlock()
	for node, event := range h.privateEvents.Data {

		switch e := event.(type) {
		case *entities.InvalidHeightEvent:
			updatedEvent := entities.NewInvalidHeightEvent(e.Node(), ts, e.Version(), e.Height())
			err := h.es.PutEvent(updatedEvent)
			if err != nil {
				h.zap.Error("Failed to put event", zap.Error(err))
				return
			}
			h.zap.Sugar().Infof("Statement 'InvalidHeight' for private node %s has been put into the storage, height %d\n", node, e.Height())
		case *entities.StateHashEvent:
			updatedEvent := entities.NewStateHashEvent(e.Node(), ts, e.Version(), e.Height(), e.StateHash(), e.BaseTarget())
			err := h.es.PutEvent(updatedEvent)
			if err != nil {
				h.zap.Error("Failed to put event", zap.Error(err))
				return
			}
			h.zap.Sugar().Infof("Statement for private node %s has been put into the storage, height %d, statehash %s\n", node, e.Height(), e.StateHash().SumHash.Hex())
		}

	}
}

func (h *PrivateNodesHandler) HandlePrivateEvents(wrappedNotifications <-chan entities.WrappedNotification, notifications chan<- entities.Notification) {
	for wn := range wrappedNotifications {
		switch wrappedNotificationType := wn.(type) {
		case *entities.OnPollingComplete:
			h.putPrivateNodesEvents(wrappedNotificationType.Ts)
			notifications <- wrappedNotificationType
		default:
			h.zap.Error("unknown analyzer notification", zap.String("type", fmt.Sprintf("%T", wrappedNotificationType)))
		}
	}
}
