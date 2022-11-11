package scraping

import (
	"context"
	"sync"
	"time"

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

func (s *Scraper) Start(ctx context.Context) <-chan entities.WrappedNotification {
	out := make(chan entities.WrappedNotification)
	go func(notifications chan<- entities.WrappedNotification) {
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

func (s *Scraper) poll(ctx context.Context, notifications chan<- entities.WrappedNotification, now int64) {
	cc, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	ec := make(chan entities.Event)
	defer close(ec)

	enabledNodes, err := s.ns.EnabledNodes()
	if err != nil {
		s.zap.Error("Failed to get nodes from storage", zap.Error(err))
	}
	wg.Add(len(enabledNodes))
	for i := range enabledNodes {
		n := enabledNodes[i]
		go func() {
			s.queryNode(cc, n.URL, ec, now)
		}()
	}

	go func() {
		cnt := 0
		for e := range ec {
			if err := s.es.PutEvent(e); err != nil {
				s.zap.Sugar().Errorf("Failed to collect event '%T' from node %s: %v", e, e.Node(), err)
			}
			switch e.(type) {
			case *entities.UnreachableEvent, *entities.InvalidHeightEvent, *entities.StateHashEvent:
				wg.Done()
			}
			cnt++
		}
		s.zap.Sugar().Infof("Polling of %d nodes completed with %d events collected", len(enabledNodes), cnt)
	}()
	wg.Wait()
	urls := make([]string, len(enabledNodes))
	for i := range enabledNodes {
		urls[i] = enabledNodes[i].URL
	}

	notifications <- entities.NewOnPollingComplete(urls, now)
}

func (s *Scraper) queryNode(ctx context.Context, url string, events chan entities.Event, ts int64) {
	node := newNodeClient(url, s.timeout, s.zap)
	v, err := node.version(ctx)
	if err != nil {
		events <- entities.NewUnreachableEvent(url, ts)
		return
	}
	events <- entities.NewVersionEvent(url, ts, v)
	h, err := node.height(ctx)
	if err != nil {
		return
	}
	if h < 2 {
		events <- entities.NewInvalidHeightEvent(url, ts, v, h)
		return
	}
	events <- entities.NewHeightEvent(url, ts, v, h)

	bs, err := node.baseTarget(ctx, h)
	if err != nil {
		return
	}
	events <- entities.NewBaseTargetEvent(url, ts, v, h, bs)

	h = h - 1 // Go to previous height to request state hash
	sh, err := node.stateHash(ctx, h)
	if err != nil {
		return
	}
	events <- entities.NewStateHashEvent(url, ts, v, h, sh, bs)

}
