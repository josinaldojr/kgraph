package server

import (
	"context"
	"log"
	"time"

	"kgraph/internal/store"
)

// pollInterval is how often the poller checks build_meta.last_build_at —
// a single indexed-by-primary-key row, so this is cheap even at a short
// interval. See design.md Decision 2 and its accepted staleness trade-off.
const pollInterval = 2 * time.Second

// Poller periodically checks whether repoPath's build_meta row has moved
// since it last looked and, when it has, reloads the graph into a fresh
// Snapshot and notifies onUpdate (the SSE broadcaster).
type Poller struct {
	ro       *store.ReadOnlyStore
	repoPath string
	gs       *GraphService
	onUpdate func()

	lastSeen int64
}

// NewPoller returns a Poller that has not yet loaded anything — call Run
// to start it; Run performs an initial load before its first tick, so the
// server has data as soon as Run returns control to its caller's goroutine
// (i.e. immediately after the first successful checkOnce).
func NewPoller(ro *store.ReadOnlyStore, repoPath string, gs *GraphService, onUpdate func()) *Poller {
	return &Poller{ro: ro, repoPath: repoPath, gs: gs, onUpdate: onUpdate, lastSeen: -1}
}

// Run polls until ctx is canceled. It performs one synchronous check
// immediately (so the very first request isn't served against an empty
// GraphService), then ticks every pollInterval. Intended to be run in its
// own goroutine.
func (p *Poller) Run(ctx context.Context) {
	p.checkOnce()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.checkOnce()
		}
	}
}

func (p *Poller) checkOnce() {
	ts, ok, err := p.ro.LastBuildAt(p.repoPath)
	if err != nil {
		log.Printf("kgraph serve: polling build_meta: %v", err)
		return
	}
	if !ok || ts == p.lastSeen {
		return
	}
	p.lastSeen = ts

	snap, err := LoadSnapshot(p.ro)
	if err != nil {
		log.Printf("kgraph serve: reloading graph: %v", err)
		return
	}
	p.gs.Store(snap)
	if p.onUpdate != nil {
		p.onUpdate()
	}
}
