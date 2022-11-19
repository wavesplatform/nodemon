package scraping

import (
	"context"
	"sync"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
)

type Scraper struct {
	ns       *nodes.Storage
	es       *events.Storage
	interval time.Duration
	timeout  time.Duration
	zap      *zap.Logger
}

func NewScraper(ns *nodes.Storage, es *events.Storage, interval, timeout time.Duration, logger *zap.Logger) (*Scraper, error) {
	return &Scraper{ns: ns, es: es, interval: interval, timeout: timeout, zap: logger}, nil
}

func (s *Scraper) Start(ctx context.Context) (<-chan entities.Notification, *atomic.Int64) {
	specificNodesTs := new(atomic.Int64)
	out := make(chan entities.Notification)
	go func(notifications chan<- entities.Notification) {
		ticker := time.NewTicker(s.interval)
		defer func() {
			ticker.Stop()
			close(notifications)
		}()
		for {
			now := time.Now().Unix()
			specificNodesTs.Store(now)
			s.poll(ctx, notifications, now)
			select {
			case <-ticker.C:
				continue
			case <-ctx.Done():
				return
			}
		}
	}(out)
	return out, specificNodesTs
}

func (s *Scraper) poll(ctx context.Context, notifications chan<- entities.Notification, now int64) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	enabledNodes, err := s.ns.EnabledNodes()
	if err != nil {
		s.zap.Error("Failed to get nodes from storage", zap.Error(err))
	}

	ec := s.queryNodes(ctx, enabledNodes, now)
	cnt := 0
	for e := range ec {
		if err := s.es.PutEvent(e); err != nil {
			s.zap.Sugar().Errorf("Failed to collect event '%T' from node %s, statement=%+v: %v", e, e.Node(), e.Statement(), err)
		} else {
			cnt++
		}
	}
	s.zap.Sugar().Infof("Polling of %d nodes completed with %d events saved", len(enabledNodes), cnt)

	urls := make([]string, len(enabledNodes))
	for i := range enabledNodes {
		urls[i] = enabledNodes[i].URL
	}
	notifications <- entities.NewOnPollingComplete(urls, now)
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
				s.zap.Sugar().Infof("Collected event (%T) for node %s", event, nodeURL)
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
		s.zap.Sugar().Warnf("Failed to get version for node %s: %v", url, err)
		return entities.NewUnreachableEvent(url, ts)
	}
	h, err := node.height(ctx)
	if err != nil {
		s.zap.Sugar().Warnf("Failed to get height for node %s: %v", url, err)
		return entities.NewVersionEvent(url, ts, v) // we know version, sending what we know about node
	}
	if h < 2 {
		s.zap.Sugar().Warnf("Node %s has invalid height %d", url, h)
		return entities.NewInvalidHeightEvent(url, ts, v, h)
	}

	// TODO: request base target for h-1 block
	bs, err := node.baseTarget(ctx, h)
	if err != nil {
		s.zap.Sugar().Warnf("Failed to get base target for node %s: %v", url, err)
		return entities.NewHeightEvent(url, ts, v, h) // we know about version and height, sending it
	}

	h = h - 1 // Go to previous height to request state hash
	sh, err := node.stateHash(ctx, h)
	if err != nil {
		s.zap.Sugar().Warnf("Failed to get state hash for node %s: %v", url, err)
		return entities.NewBaseTargetEvent(url, ts, v, h, bs) // we know version, height and base target, sending it
	}
	return entities.NewStateHashEvent(url, ts, v, h, sh, bs) // sending full info about node
}
