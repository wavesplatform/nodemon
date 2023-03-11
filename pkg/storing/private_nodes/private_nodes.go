package private_nodes

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
)

type PrivateNodesEventsWriter interface {
	Write(event entities.EventProducerWithTimestamp)
}

type privateNodesEvents struct {
	mu   *sync.RWMutex
	data map[string]entities.EventProducerWithTimestamp // map[node]NodeStatement
}

func newPrivateNodesEvents() *privateNodesEvents {
	return &privateNodesEvents{
		mu:   new(sync.RWMutex),
		data: make(map[string]entities.EventProducerWithTimestamp),
	}
}

func (p *privateNodesEvents) Write(producer entities.EventProducerWithTimestamp) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.unsafeWrite(producer)
}

func (p *privateNodesEvents) unsafeWrite(producer entities.EventProducerWithTimestamp) {
	p.data[producer.Node()] = producer
}

type PrivateNodesHandler struct {
	es            *events.Storage
	zap           *zap.Logger
	privateEvents *privateNodesEvents
}

func NewPrivateNodesHandlerWithUnreachableInitialState(es *events.Storage, ns nodes.Storage, zap *zap.Logger) (*PrivateNodesHandler, error) {
	privateNodes, err := ns.Nodes(true) // get private nodes aka specific nodes
	if err != nil {
		return nil, errors.Wrap(err, "failed to get specific nodes")
	}
	initialTS := time.Now().Unix()
	initialPrivateNodesEvents := make([]entities.EventProducerWithTimestamp, len(privateNodes))
	for i, node := range privateNodes {
		initialPrivateNodesEvents[i] = entities.NewUnreachableEvent(node.URL, initialTS)
	}
	return NewPrivateNodesHandler(es, zap, initialPrivateNodesEvents...), nil
}

func NewPrivateNodesHandler(es *events.Storage, zap *zap.Logger, initial ...entities.EventProducerWithTimestamp) *PrivateNodesHandler {
	pe := newPrivateNodesEvents()
	for _, producer := range initial {
		pe.unsafeWrite(producer)
	}
	return &PrivateNodesHandler{
		es:            es,
		zap:           zap,
		privateEvents: pe,
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
		if err := h.putPrivateNodesEvent(event); err != nil {
			h.zap.Error("Failed to put private node event", zap.Error(err))
			return nodes
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (h *PrivateNodesHandler) putPrivateNodesEvent(e entities.Event) error {
	if err := h.es.PutEvent(e); err != nil {
		return errors.Wrapf(err, "failed to put event (%T) for node %s", e, e.Node())
	}
	switch e := e.(type) {
	case *entities.InvalidHeightEvent:
		h.zap.Sugar().Infof("Statement produced by (%T) for private node %s has been put into the storage, height %d", e, e.Node(), e.Height())
	case *entities.StateHashEvent:
		h.zap.Sugar().Infof("Statement produced by (%T) for private node %s has been put into the storage, height %d, statehash %s",
			e, e.Node(), e.Height(), e.StateHash().SumHash.Hex(),
		)
	default:
		h.zap.Sugar().Warnf("Statement produced by unexpected (%T) for private node %s has been put into the storage", e, e.Node())
	}
	return nil
}

func (h *PrivateNodesHandler) Run(input <-chan entities.NodesGatheringNotification) <-chan entities.NodesGatheringNotification {
	output := make(chan entities.NodesGatheringNotification)
	go h.handlePrivateEvents(input, output)
	return output
}

func (h *PrivateNodesHandler) handlePrivateEvents(input <-chan entities.NodesGatheringNotification, output chan<- entities.NodesGatheringNotification) {
	defer close(output)
	for notification := range input {
		var (
			ts          = notification.Timestamp()
			polledNodes = notification.Nodes()
		)
		storedPrivateNodes := h.putPrivateNodesEvents(ts)
		h.zap.Sugar().Infof("Total count of stored private nodes statements is %d at timestamp %d", len(storedPrivateNodes), ts)
		output <- entities.NewNodesGatheringComplete(append(polledNodes, storedPrivateNodes...), ts)
	}
}
