package scraping

import (
	"context"
	"encoding/json"
	"go.nanomsg.org/mangos/v3/protocol"
	"log"
	"nodemon/pkg/common"
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

type invalidHeightEvent struct {
	n string
	h int
}

func (e *invalidHeightEvent) node() string {
	return e.n
}

type stateHashEvent struct {
	n  string
	h  int
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

func (s *Scraper) Start(ctx context.Context, socket protocol.Socket) {
	go func() {
		ticker := time.NewTicker(s.interval)
		for {
			s.poll(ctx, socket)
			select {
			case <-ticker.C:
				continue
			case <-ctx.Done():
				return
			}
		}
	}()


}

func (s *Scraper) poll(ctx context.Context, socket protocol.Socket) {
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

				event := &common.TimeoutEvent{Node: e.node()}
				message, err := json.Marshal(event)
				if err != nil {
					log.Printf("failed to marshal json, %v", err)
				}
				message = common.AddTypeByte(message, common.TimeoutEvnt)
				if err := socket.Send(message); err != nil {
					log.Printf("failed to sent message to telegram, %v", err)
				}

				wg.Done()
			case *versionEvent:
				log.Printf("[%s] Version: %s", e.node(), te.v)

				event := &common.VersionEvent{Node: e.node(), Version: te.v}
				message, err := json.Marshal(event)
				if err != nil {
					log.Printf("failed to marshal json, %v", err)
				}
				message = common.AddTypeByte(message, common.VersionEvnt)
				if err := socket.Send(message); err != nil {
					log.Printf("failed to sent message to telegram, %v", err)
				}

			case *heightEvent:
				log.Printf("[%s] Height: %d", e.node(), te.h)

				event := &common.HeightEvent{Node: e.node(), Height: te.h}
				message, err := json.Marshal(event)
				if err != nil {
					log.Printf("failed to marshal json, %v", err)
				}
				message = common.AddTypeByte(message, common.HeightEvnt)

				if err := socket.Send(message); err != nil {
					log.Printf("failed to sent message to telegram, %v", err)
				}

			case *invalidHeightEvent:
				log.Printf("[%s] Invalid height: %d", e.node(), te.h)

				event := &common.InvalidHeightEvent{Node: e.node(), Height: te.h}
				message, err := json.Marshal(event)
				if err != nil {
					log.Printf("failed to marshal json, %v", err)
				}
				message = common.AddTypeByte(message, common.InvalidHeightEvnt)
				if err := socket.Send([]byte(e.node())); err != nil {
					log.Printf("failed to sent message to telegram, %v", err)
				}

			case *stateHashEvent:
				log.Printf("[%s] State Hash of block '%s' at height %d: %s",
					e.node(), te.sh.BlockID.ShortString(), te.h, te.sh.SumHash.ShortString())

				event := &common.StateHashEvent{Node: e.node(), Height: te.h, StateHash: te.sh}
				message, err := json.Marshal(event)
				if err != nil {
					log.Printf("failed to marshal json, %v", err)
				}
				message = common.AddTypeByte(message, common.StateHashEvnt)
				if err := socket.Send(message); err != nil {
					log.Printf("failed to sent message to telegram, %v", err)
				}
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
	if h < 2 {
		events <- &invalidHeightEvent{n: url, h: h}
		return
	}
	events <- &heightEvent{n: url, h: h}
	h = h - 1 // Go to prev height to request state hash
	sh, err := node.stateHash(ctx, h)
	if err != nil {
		events <- &timeoutEvent{n: url}
		return
	}
	events <- &stateHashEvent{n: url, h: h, sh: sh}
}
