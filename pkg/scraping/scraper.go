package scraping

import (
	"context"
	stderrs "errors"
	"sync"
	"time"

	"github.com/pkg/errors"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"

	"go.uber.org/zap"
)

type Scraper struct {
	ns       nodes.Storage
	es       *events.Storage
	interval time.Duration
	timeout  time.Duration
	zap      *zap.Logger
}

func NewScraper(
	ns nodes.Storage,
	es *events.Storage,
	interval, timeout time.Duration,
	logger *zap.Logger,
) (*Scraper, error) {
	return &Scraper{ns: ns, es: es, interval: interval, timeout: timeout, zap: logger}, nil
}

func (s *Scraper) Start(ctx context.Context) <-chan entities.NodesGatheringNotification {
	out := make(chan entities.NodesGatheringNotification)
	go func(notifications chan<- entities.NodesGatheringNotification) {
		ticker := time.NewTicker(s.interval)
		defer func() {
			ticker.Stop()
			close(notifications)
		}()
		for {
			now := time.Now().Unix()
			s.poll(ctx, notifications, now)
			select {
			case <-ticker.C:
				continue
			case <-ctx.Done():
				return
			}
		}
	}(out)
	return out
}

func (s *Scraper) poll(ctx context.Context, notifications chan<- entities.NodesGatheringNotification, now int64) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	enabledNodes, storageErr := s.ns.EnabledNodes()
	if storageErr != nil {
		s.zap.Error("[SCRAPER] Failed to get nodes from storage", zap.Error(storageErr))
		notifications <- entities.NewNodesGatheringError(
			errors.Wrapf(storageErr, "scraper: failed to get nodes from storage"), now,
		)
	}

	ec := s.queryNodes(ctx, enabledNodes, now)
	cnt := 0
	var errs []error
	for e := range ec {
		if err := s.es.PutEvent(e); err != nil {
			s.zap.Sugar().Errorf("[SCRAPER] Failed to collect event '%T' from node %s, statement=%+v: %v",
				e, e.Node(), e.Statement(), err)
			errs = append(errs, errors.Wrapf(err, "scraper: failed to collect event '%T' from node %s: %v",
				e, e.Node(), err,
			))
		} else {
			cnt++
		}
	}
	if len(errs) > 0 {
		sumErr := errors.Wrapf(stderrs.Join(errs...), "scraper: failed to collect %d events", len(errs))
		notifications <- entities.NewNodesGatheringError(sumErr, now)
		return
	}
	s.zap.Sugar().Infof("[SCRAPER] Polling of %d nodes completed with %d events saved", len(enabledNodes), cnt)

	urls := make([]string, len(enabledNodes))
	for i := range enabledNodes {
		urls[i] = enabledNodes[i].URL
	}
	notifications <- entities.NewNodesGatheringComplete(urls, now)
}

func (s *Scraper) queryNodes(ctx context.Context, nodes []entities.Node, now int64) <-chan entities.Event {
	poller := func(ec chan<- entities.Event) {
		wg := new(sync.WaitGroup)
		defer func() {
			wg.Wait()
			close(ec)
		}()
		wg.Add(len(nodes))
		for i := range nodes {
			nodeURL := nodes[i].URL
			go func() {
				defer wg.Done()
				event := s.queryNode(ctx, nodeURL, now)
				s.zap.Sugar().Infof("[SCRAPER] Collected event (%T) at height %d for node %s",
					event, event.Height(), nodeURL,
				)
				ec <- event
			}()
		}
	}
	ec := make(chan entities.Event, len(nodes))
	go poller(ec)
	return ec
}

func (s *Scraper) queryNode(ctx context.Context, url string, ts int64) entities.Event {
	node := newNodeClient(url, s.timeout, s.zap)
	v, err := node.version(ctx)
	if err != nil {
		s.zap.Sugar().Warnf("[SCRAPER] Failed to get version for node %s: %v", url, err)
		return entities.NewUnreachableEvent(url, ts)
	}
	s.zap.Sugar().Debugf("[SCRAPER] Node %s has version %s", url, v)

	h, err := node.height(ctx)
	if err != nil {
		s.zap.Sugar().Warnf("[SCRAPER] Failed to get height for node %s: %v", url, err)
		return entities.NewVersionEvent(url, ts, v) // we know version, sending what we know about node
	}
	s.zap.Sugar().Debugf("[SCRAPER] Node %s has height %d", url, h)

	const minValidHeight = 2
	if h < minValidHeight {
		s.zap.Sugar().Warnf("[SCRAPER] Node %s has invalid height %d", url, h)
		return entities.NewInvalidHeightEvent(url, ts, v, h)
	}

	blockHeader, err := node.blockHeader(ctx, h)
	if err != nil {
		s.zap.Sugar().Warnf("[SCRAPER] Failed to get block header at height %d for node %s: %v", h, url, err)
		return entities.NewHeightEvent(url, ts, v, h) // we know about version and height, sending it
	}
	var (
		blockID    = blockHeader.ID
		generator  = blockHeader.Generator
		challenged = blockHeader.ChallengedHeader != nil // if challenged header is not nil, then it's challenged
	)

	h-- // Go to previous height to request base target and state hash

	bs, err := node.baseTarget(ctx, h)
	if err != nil {
		s.zap.Sugar().Warnf("[SCRAPER] Failed to get base target at height %d for node %s: %v", h, url, err)
		// we know version, height and block generator, sending it
		return entities.NewBlockHeaderEvent(url, ts, v, h, &blockID, &generator, challenged)
	}
	s.zap.Sugar().Debugf("[SCRAPER] Node %s has base target %d at height %d", url, bs, h)

	sh, err := node.stateHash(ctx, h)
	if err != nil {
		s.zap.Sugar().Warnf("[SCRAPER] Failed to get state hash for node %s at height %d: %v", url, h, err)
		// we know version, height and base target, block generator, sending it
		return entities.NewBaseTargetEvent(url, ts, v, h, bs, &blockID, &generator, challenged)
	}
	s.zap.Sugar().Debugf("[SCRAPER] Node %s has state hash %s at height %d", url, sh.SumHash.Hex(), h)
	return entities.NewStateHashEvent(url, ts, v, h, sh, bs, &blockID, &generator, challenged) // sending full info
}
