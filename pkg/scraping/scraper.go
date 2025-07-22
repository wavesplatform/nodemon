package scraping

import (
	"context"
	stderrs "errors"
	"log/slog"
	"sync"
	"time"

	"github.com/pkg/errors"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
	"nodemon/pkg/tools/logging/attrs"
)

type Scraper struct {
	ns       nodes.Storage
	es       *events.Storage
	interval time.Duration
	timeout  time.Duration
	logger   *slog.Logger
}

func NewScraper(
	ns nodes.Storage,
	es *events.Storage,
	interval, timeout time.Duration,
	logger *slog.Logger,
) (*Scraper, error) {
	// TODO: add SCRAPER nambespace to logger
	return &Scraper{ns: ns, es: es, interval: interval, timeout: timeout, logger: logger}, nil
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
		s.logger.Error("[SCRAPER] Failed to get nodes from storage", attrs.Error(storageErr))
		notifications <- entities.NewNodesGatheringError(
			errors.Wrapf(storageErr, "scraper: failed to get nodes from storage"), now,
		)
	}

	ec := s.queryNodes(ctx, enabledNodes, now)
	cnt := 0
	var errs []error
	for e := range ec {
		if err := s.es.PutEvent(e); err != nil {
			s.logger.Error("[SCRAPER] Failed to collect event from node",
				attrs.Type(e), slog.String("node", e.Node()),
				slog.Any("statement", e.Statement()), attrs.Error(err),
			)
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
	s.logger.Info("[SCRAPER] Polling of nodes completed, events saved",
		slog.Int("nodes_count", len(enabledNodes)), slog.Int("events_count", cnt),
	)

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
				s.logger.Info("[SCRAPER] Collected event at height for node", attrs.Type(event),
					slog.Uint64("height", event.Height()), slog.String("node", nodeURL),
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
	node := newNodeClient(url, s.timeout, s.logger)
	v, err := node.version(ctx)
	if err != nil {
		s.logger.Warn("[SCRAPER] Failed to get version for node", slog.String("node", url), attrs.Error(err))
		return entities.NewUnreachableEvent(url, ts)
	}
	s.logger.Debug("[SCRAPER] Received version from node",
		slog.String("node", url), slog.String("version", v),
	)

	h, err := node.height(ctx)
	if err != nil {
		s.logger.Warn("[SCRAPER] Failed to get height for node", slog.String("node", url), attrs.Error(err))
		return entities.NewVersionEvent(url, ts, v) // we know a version, sending what we know about node
	}
	s.logger.Debug("[SCRAPER] Received node height", slog.String("node", url), slog.Uint64("height", h))

	const minValidHeight = 2
	if h < minValidHeight {
		s.logger.Warn("[SCRAPER] Node has invalid height", slog.String("node", url), slog.Uint64("height", h))
		return entities.NewInvalidHeightEvent(url, ts, v, h)
	}

	blockHeader, err := node.blockHeader(ctx, h)
	if err != nil {
		s.logger.Warn("[SCRAPER] Failed to get block header for node",
			slog.Uint64("height", h), slog.String("node", url), attrs.Error(err),
		)
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
		s.logger.Warn("[SCRAPER] Failed to get base target for node",
			slog.Uint64("height", h), slog.String("node", url), attrs.Error(err),
		)
		// we know version, height and block generator, sending it
		return entities.NewBlockHeaderEvent(url, ts, v, h, &blockID, &generator, challenged)
	}
	s.logger.Debug("[SCRAPER] Received base target from node",
		slog.String("node", url), slog.Uint64("base_target", bs), slog.Uint64("height", h),
	)

	sh, err := node.stateHash(ctx, h)
	if err != nil {
		s.logger.Warn("[SCRAPER] Failed to get state hash for node",
			slog.Uint64("height", h), slog.String("node", url), attrs.Error(err),
		)
		// we know version, height and base target, block generator, sending it
		return entities.NewBaseTargetEvent(url, ts, v, h, bs, &blockID, &generator, challenged)
	}
	s.logger.Debug("[SCRAPER] Received state hash from node",
		slog.String("node", url), slog.String("state_hash", sh.SumHash.Hex()), slog.Uint64("height", h),
	)
	return entities.NewStateHashEvent(url, ts, v, h, sh, bs, &blockID, &generator, challenged) // sending full info
}
