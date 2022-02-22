package scraping

import (
	"context"
	"log"
	"sync"
	"time"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
)

type Scraper struct {
	ns       *nodes.Storage
	es       *events.Storage
	interval time.Duration
	timeout  time.Duration
}

func NewScraper(ns *nodes.Storage, es *events.Storage, interval, timeout time.Duration) (*Scraper, error) {
	return &Scraper{ns: ns, es: es, interval: interval, timeout: timeout}, nil
}

func (s *Scraper) Start(ctx context.Context) <-chan entities.Notification {
	out := make(chan entities.Notification)
	go func(notifications chan<- entities.Notification) {
		ticker := time.NewTicker(s.interval)
		defer func() {
			ticker.Stop()
			close(notifications)
		}()
		for {
			s.poll(ctx, notifications)
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

func (s *Scraper) poll(ctx context.Context, notifications chan<- entities.Notification) {
	ts := time.Now().Unix()
	cc, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	ec := make(chan entities.Event)
	defer close(ec)

	ns, err := s.ns.EnabledNodes()
	if err != nil {
		log.Printf("Failed to get nodes from storage: %v", err)
	}
	wg.Add(len(ns))
	for i := range ns {
		n := ns[i]
		go func() {
			s.queryNode(cc, n.URL, ec, ts)
		}()
	}
	go func() {
		cnt := 0
		for e := range ec {
			if err := s.es.PutEvent(e); err != nil {
				log.Printf("Failed to collect event '%T' from node %s: %v", e, e.Node(), err)
			}
			switch e.(type) {
			case *entities.UnreachableEvent, *entities.InvalidHeightEvent, *entities.StateHashEvent:
				wg.Done()
			}
			cnt++
		}
		log.Printf("Polling of %d nodes completed with %d events collected", len(ns), cnt)
	}()
	wg.Wait()
	urls := make([]string, len(ns))
	for i := range ns {
		urls[i] = ns[i].URL
	}
	notifications <- entities.NewOnPollingComplete(urls, ts)
}

func (s *Scraper) queryNode(ctx context.Context, url string, events chan entities.Event, ts int64) {
	node := newNodeClient(url, s.timeout)
	v, err := node.version(ctx)
	if err != nil {
		events <- entities.NewUnreachableEvent(url, ts)
		return
	}
	events <- entities.NewVersionEvent(url, ts, v)
	h, err := node.height(ctx)
	if err != nil {
		events <- entities.NewUnreachableEvent(url, ts)
		return
	}
	if h < 2 {
		events <- entities.NewInvalidHeightEvent(url, ts, v, h)
		return
	}
	events <- entities.NewHeightEvent(url, ts, v, h)
	h = h - 1 // Go to previous height to request state hash
	sh, err := node.stateHash(ctx, h)
	if err != nil {
		events <- entities.NewUnreachableEvent(url, ts)
		return
	}
	events <- entities.NewStateHashEvent(url, ts, v, h, sh)
}
