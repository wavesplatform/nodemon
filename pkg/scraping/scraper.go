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
	interval time.Duration
	timeout  time.Duration
}

func NewScraper(ns *storing.NodesStorage, interval, timeout time.Duration) (*Scraper, error) {
	return &Scraper{ns: ns, interval: interval, timeout: timeout}, nil
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
			s.queryNode(cc, n.URL, events)
		}()
	}
	go func() {
		for e := range events {
			// PUT events into ts storage here
			switch te := e.(type) {
			case *entities.UnreachableEvent:
				log.Printf("[%s] Unreachable", e.Node())
				wg.Done()
			case *entities.VersionEvent:
				log.Printf("[%s] Version: %s", e.Node(), te.Version())
			case *entities.HeightEvent:
				log.Printf("[%s] Height: %d", e.Node(), te.Height())
			case *entities.InvalidHeightEvent:
				log.Printf("[%s] Invalid height: %d", e.Node(), te.Height())
			case *entities.StateHashEvent:
				log.Printf("[%s] State Hash of block '%s' at height %d: %s",
					e.Node(), te.StateHash().BlockID.ShortString(), te.Height(), te.StateHash().SumHash.ShortString())
				wg.Done()
			}
		}
	}()
	wg.Wait()
}

func (s *Scraper) queryNode(ctx context.Context, url string, events chan entities.Event) {
	node := newNodeClient(url, s.timeout)
	v, err := node.version(ctx)
	if err != nil {
		events <- entities.NewUnreachableEvent(url)
		return
	}
	events <- entities.NewVersionEvent(url, v)
	h, err := node.height(ctx)
	if err != nil {
		events <- entities.NewUnreachableEvent(url)
		return
	}
	if h < 2 {
		events <- entities.NewInvalidHeightEvent(url, h)
		return
	}
	events <- entities.NewHeightEvent(url, h)
	h = h - 1 // Go to previous height to request state hash
	sh, err := node.stateHash(ctx, h)
	if err != nil {
		events <- entities.NewUnreachableEvent(url)
		return
	}
	events <- entities.NewStateHashEvent(url, h, sh)
}
