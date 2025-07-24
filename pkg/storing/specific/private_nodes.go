package specific

import (
	stderrs "errors"
	"log/slog"
	"maps"
	"sync"
	"time"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
	"nodemon/pkg/tools/logging/attrs"

	"github.com/pkg/errors"
)

type PrivateNodesEventsWriter interface {
	Write(event entities.EventProducerWithTimestamp)
	WriteInitialStateForSpecificNode(node string, ts int64)
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

func (p *privateNodesEvents) WriteInitialStateForSpecificNode(node string, ts int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.unsafeWriteInitialStateForSpecificNodes(node, ts)
}

func (p *privateNodesEvents) unsafeWriteInitialStateForSpecificNodes(node string, ts int64) {
	if _, ok := p.data[node]; ok { // already exists
		return
	}
	p.unsafeWrite(entities.NewUnreachableEvent(node, ts))
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
	logger        *slog.Logger
	privateEvents *privateNodesEvents
}

func NewPrivateNodesHandlerWithUnreachableInitialState(
	es *events.Storage,
	ns nodes.Storage,
	logger *slog.Logger,
) (*PrivateNodesHandler, error) {
	privateNodes, err := ns.EnabledSpecificNodes() // get private nodes aka specific nodes
	if err != nil {
		return nil, errors.Wrap(err, "failed to get specific nodes")
	}
	ts := time.Now().Unix()
	h := NewPrivateNodesHandler(es, ns, logger)
	for _, node := range privateNodes {
		h.privateEvents.unsafeWriteInitialStateForSpecificNodes(node.URL, ts)
	}
	return h, nil
}

func NewPrivateNodesHandler(
	es *events.Storage,
	ns nodes.Storage,
	logger *slog.Logger,
	initial ...entities.EventProducerWithTimestamp,
) *PrivateNodesHandler {
	return &PrivateNodesHandler{
		es:            es,
		ns:            ns,
		logger:        logger,
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

func (h *PrivateNodesHandler) putPrivateNodesEvent(ev entities.Event) error {
	if err := h.es.PutEvent(ev); err != nil {
		return errors.Wrapf(err, "failed to put event (%T) for node %s", ev, ev.Node())
	}
	const msg = "Statement produced for private node has been put into the storage"
	switch e := ev.(type) {
	case *entities.InvalidHeightEvent:
		h.logger.Info(msg, attrs.Type(e), slog.String("node", e.Node()), slog.Uint64("height", e.Height()))
	case *entities.StateHashEvent:
		h.logger.Info(msg,
			attrs.Type(e), slog.String("node", e.Node()), slog.Uint64("height", e.Height()),
			slog.String("statehash", e.StateHash().SumHash.Hex()),
		)
	default:
		h.logger.Warn(
			"Unexpected statement produced for private node has been put into the storage",
			attrs.Type(e), slog.String("node", e.Node()),
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
			h.logger.Error("Failed to filter private nodes", attrs.Error(err))
			output <- entities.NewNodesGatheringError(err, ts) // pass through error, but continue processing
		}
		storedPrivateNodes, err := h.putPrivateNodesEvents(ts)
		h.logger.Info("Total private nodes events stored",
			slog.Int("private_nodes_count", len(storedPrivateNodes)), slog.Int64("timestamp", ts),
		)
		if len(storedPrivateNodes) > 0 {
			polledNodes := notification.Nodes()
			notification = entities.NewNodesGatheringComplete(append(polledNodes, storedPrivateNodes...), ts)
		}
		if err != nil {
			h.logger.Error("Failed to put some private nodes events", attrs.Error(err))
			notification = entities.NewNodesGatheringWithError(notification, err) // pass through error
		}
		output <- notification
	}
}
