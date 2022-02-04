package scraping

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/storing"
)

type event interface {
	node() string
}

type timeoutEvent struct {
	n string
}

func (e *timeoutEvent) node() string {
	return e.n
}

type versionEvent struct {
	n string
	v string
}

func (e *versionEvent) node() string {
	return e.n
}

type heightEvent struct {
	n string
	h int
}

func (e *heightEvent) node() string {
	return e.n
}

type stateHashEvent struct {
	n  string
	sh *proto.StateHash
}

func (e *stateHashEvent) node() string {
	return e.n
}

type Scraper struct {
	cs       *storing.ConfigurationStorage
	interval time.Duration
	timeout  time.Duration
}

func NewScraper(cs *storing.ConfigurationStorage, interval, timeout time.Duration) (*Scraper, error) {
	return &Scraper{cs: cs, interval: interval, timeout: timeout}, nil
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
	events := make(chan event)
	defer close(events)

	nodes, err := s.cs.EnabledNodes()
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
			case *timeoutEvent:
				log.Printf("[%s] Timeout", e.node())
				wg.Done()
			case *versionEvent:
				log.Printf("[%s] Version: %s", e.node(), te.v)
			case *heightEvent:
				log.Printf("[%s] Height: %d", e.node(), te.h)
			case *stateHashEvent:
				log.Printf("[%s] State Hash of block '%s': %s", e.node(), te.sh.BlockID.ShortString(), te.sh.SumHash.ShortString())
				wg.Done()
			}
		}
	}()
	wg.Wait()
}

func (s *Scraper) queryNode(ctx context.Context, url string, events chan event) {
	node := newNodeClient(url, s.timeout)
	v, err := node.version(ctx)
	if err != nil {
		events <- &timeoutEvent{n: url}
		return
	}
	events <- &versionEvent{n: url, v: v}
	h, err := node.height(ctx)
	if err != nil {
		events <- &timeoutEvent{n: url}
		return
	}
	events <- &heightEvent{n: url, h: h}
	sh, err := node.stateHash(ctx, h-1)
	if err != nil {
		events <- &timeoutEvent{n: url}
		return
	}
	events <- &stateHashEvent{n: url, sh: sh}
}
