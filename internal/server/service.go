package server

import "sync/atomic"

// GraphService holds the current Snapshot, safe for concurrent reads from
// HTTP handlers while the poller goroutine swaps in a new one. It never
// touches the database itself — that's the poller's job — so it has no
// dependency on *store.ReadOnlyStore at all.
type GraphService struct {
	current atomic.Pointer[Snapshot]
}

// NewGraphService returns a GraphService with no snapshot loaded yet.
// Handlers must treat a nil Snapshot() as "not ready" (503), which is only
// possible for the brief window between server startup and the poller's
// first successful load.
func NewGraphService() *GraphService {
	return &GraphService{}
}

// Snapshot returns the current snapshot, or nil if none has loaded yet.
func (gs *GraphService) Snapshot() *Snapshot {
	return gs.current.Load()
}

// Store atomically replaces the current snapshot.
func (gs *GraphService) Store(snap *Snapshot) {
	gs.current.Store(snap)
}
