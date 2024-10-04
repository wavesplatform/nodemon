package specific

import (
	stderrs "errors"
	"maps"
	"sync"
	"time"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type PrivateNodesEventsWriter interface {
	Write(event entities.EventProducerWithTimestamp)
}

type privateNodesEvents struct {
	mu   *sync.RWMutex
	data map[string]entities.EventProducerWithTimestamp // map[node]NodeStatement
}

func newPrivateNodesEvents(initial ...entities.EventProducerWithTimestamp) *privateNodesEvents {
	pe := &privateNodesEvents{
		mu:   new(sync.RWMutex),
		data: make(map[string]entities.EventProducerWithTimestamp, len(initial)),
	}
	for _, producer := range initial {
		pe.unsafeWrite(producer)
	}
	return pe
}

func (p *privateNodesEvents) Write(producer entities.EventProducerWithTimestamp) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.unsafeWrite(producer)
}

func (p *privateNodesEvents) unsafeWrite(producer entities.EventProducerWithTimestamp) {
	p.data[producer.Node()] = producer
}

func (p *privateNodesEvents) filterPrivateNodes(ns nodes.Storage) error {
	specificNodesList, err := ns.EnabledSpecificNodes()
	if err != nil {
		return errors.Wrap(err, "privateNodesEvents: failed to get specific nodes")
	}
	nodesURLs := make(map[string]struct{}, len(specificNodesList))
	for _, node := range specificNodesList {
		nodesURLs[node.URL] = struct{}{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	maps.DeleteFunc(p.data, func(node string, _ entities.EventProducerWithTimestamp) bool {
		_, ok := nodesURLs[node]
		return !ok
	})
	return nil
}

type PrivateNodesHandler struct {
	es            *events.Storage
	ns            nodes.Storage
	zap           *zap.Logger
	privateEvents *privateNodesEvents
}

func NewPrivateNodesHandlerWithUnreachableInitialState(
	es *events.Storage,
	ns nodes.Storage,
	zap *zap.Logger,
) (*PrivateNodesHandler, error) {
	privateNodes, err := ns.EnabledSpecificNodes() // get private nodes aka specific nodes
	if err != nil {
		return nil, errors.Wrap(err, "failed to get specific nodes")
	}
	initialTS := time.Now().Unix()
	initialPrivateNodesEvents := make([]entities.EventProducerWithTimestamp, len(privateNodes))
	for i, node := range privateNodes {
		initialPrivateNodesEvents[i] = entities.NewUnreachableEvent(node.URL, initialTS)
	}
	return NewPrivateNodesHandler(es, ns, zap, initialPrivateNodesEvents...), nil
}

func NewPrivateNodesHandler(
	es *events.Storage,
	ns nodes.Storage,
	zap *zap.Logger,
	initial ...entities.EventProducerWithTimestamp,
) *PrivateNodesHandler {
	return &PrivateNodesHandler{
		es:            es,
		ns:            ns,
		zap:           zap,
		privateEvents: newPrivateNodesEvents(initial...),
	}
}

func (h *PrivateNodesHandler) PrivateNodesEventsWriter() PrivateNodesEventsWriter {
	return h.privateEvents
}

func (h *PrivateNodesHandler) putPrivateNodesEvents(ts int64) (entities.Nodes, error) {
	nodesList := make(entities.Nodes, 0, len(h.privateEvents.data))
	h.privateEvents.mu.RLock()
	defer h.privateEvents.mu.RUnlock()
	var errs []error
	for node, eventProducer := range h.privateEvents.data {
		event := eventProducer.WithTimestamp(ts)
		if err := h.putPrivateNodesEvent(event); err != nil {
			errs = append(errs, errors.Wrapf(err, "privateNodesHandler: failed to put event for private node %s", node))
			continue // skip failed events
		}
		nodesList = append(nodesList, node)
	}
	return nodesList, stderrs.Join(errs...)
}

func (h *PrivateNodesHandler) putPrivateNodesEvent(e entities.Event) error {
	if err := h.es.PutEvent(e); err != nil {
		return errors.Wrapf(err, "failed to put event (%T) for node %s", e, e.Node())
	}
	switch e := e.(type) {
	case *entities.InvalidHeightEvent:
		h.zap.Sugar().Infof(
			"Statement produced by (%T) for private node %s has been put into the storage, height %d",
			e, e.Node(), e.Height(),
		)
	case *entities.StateHashEvent:
		h.zap.Sugar().Infof(
			"Statement produced by (%T) for private node %s has been put into the storage, height %d, statehash %s",
			e, e.Node(), e.Height(), e.StateHash().SumHash.Hex(),
		)
	default:
		h.zap.Sugar().Warnf(
			"Statement produced by unexpected (%T) for private node %s has been put into the storage",
			e, e.Node(),
		)
	}
	return nil
}

func (h *PrivateNodesHandler) Run(
	input <-chan entities.NodesGatheringNotification,
) <-chan entities.NodesGatheringNotification {
	output := make(chan entities.NodesGatheringNotification)
	go h.handlePrivateEvents(input, output)
	return output
}

func (h *PrivateNodesHandler) handlePrivateEvents(
	input <-chan entities.NodesGatheringNotification,
	output chan<- entities.NodesGatheringNotification,
) {
	defer close(output)
	for notification := range input {
		if notification.Error() != nil { // pass through error notifications
			output <- notification
			continue
		}
		ts := notification.Timestamp()
		if err := h.privateEvents.filterPrivateNodes(h.ns); err != nil {
			h.zap.Error("Failed to filter private nodes", zap.Error(err))
			output <- entities.NewNodesGatheringError(err, ts) // pass through error, but continue processing
		}
		storedPrivateNodes, err := h.putPrivateNodesEvents(ts)
		h.zap.Sugar().Infof("Total count of stored private nodes statements is %d at timestamp %d",
			len(storedPrivateNodes), ts,
		)
		if len(storedPrivateNodes) > 0 {
			polledNodes := notification.Nodes()
			notification = entities.NewNodesGatheringComplete(append(polledNodes, storedPrivateNodes...), ts)
		}
		if err != nil {
			h.zap.Error("Failed to put some private nodes events", zap.Error(err))
			notification = entities.NewNodesGatheringWithError(notification, err) // pass through error
		}
		output <- notification
	}
}
