package scraping

import (
	"context"
	"log"
	"sync"
	"time"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing"
)

type Scraper struct {
	ns       *storing.NodesStorage
	es       *storing.EventsStorage
	interval time.Duration
	timeout  time.Duration
}

func NewScraper(ns *storing.NodesStorage, es *storing.EventsStorage, interval, timeout time.Duration) (*Scraper, error) {
	return &Scraper{ns: ns, es: es, interval: interval, timeout: timeout}, nil
}

func (s *Scraper) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.interval)
		for {
			s.poll(ctx)
			select {
			case <-ticker.C:
				continue
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Scraper) poll(ctx context.Context) {
	ts := time.Now().Unix()
	cc, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	events := make(chan entities.Event)
	defer close(events)

	nodes, err := s.ns.EnabledNodes()
	if err != nil {
		log.Printf("Failed to get nodes from storage: %v", err)
	}
	wg.Add(len(nodes))
	for i := range nodes {
		n := nodes[i]
		go func() {
			s.queryNode(cc, n.URL, events, ts)
		}()
	}
	go func() {
		cnt := 0
		for e := range events {
			if err := s.es.PutEvent(e); err != nil {
				log.Printf("Failed to collect event '%T' from node %s: %v", e, e.Node(), err)
			}
			switch e.(type) {
			case *entities.UnreachableEvent, *entities.InvalidHeightEvent, *entities.StateHashEvent:
				wg.Done()
			}
			cnt++
		}
		log.Printf("Polling of %d nodes completed with %d events collected", len(nodes), cnt)
	}()
	wg.Wait()
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
