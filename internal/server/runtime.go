package server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/josinaldojr/kgraph/internal/store"
)

// ErrUnknownProject signals that a request targeted a project key matching
// no discovered database. Hub routes turn it into a clear JSON 404 rather
// than an empty or misleading graph.
var ErrUnknownProject = errors.New("server: unknown project")

// ErrProjectUnreadable signals that the requested project's database exists
// but cannot be opened (corrupt file). Distinct from ErrUnknownProject so
// hub routes can say why a known key still can't be served.
var ErrProjectUnreadable = errors.New("server: project database is unreadable")

// ErrRegistryClosed signals that the hub is shutting down and cannot safely
// create a new runtime. It prevents a request racing shutdown from leaving a
// newly opened read-only store outside Shutdown's stop set.
var ErrRegistryClosed = errors.New("server: project registry is shut down")

// ProjectRuntime is one project's serving stack: a read-only store kept
// fresh by its own Poller, an atomic snapshot holder, and an SSE fan-out —
// the same composition single-project mode assembles in cmd_serve.go, but
// created lazily per project and scoped so one project's rebuild never
// touches another's streams. The cancel func and done channel tie the
// poller goroutine's lifetime to server shutdown.
type ProjectRuntime struct {
	Info   store.ProjectInfo
	RO     *store.ReadOnlyStore
	GS     *GraphService
	BC     *Broadcaster
	cancel context.CancelFunc
	done   chan struct{}
}

// stop cancels the poller, waits for it to exit, and closes the read-only
// store. Safe to call once; the registry calls it on shutdown.
func (rt *ProjectRuntime) stop() {
	rt.cancel()
	<-rt.done
	_ = rt.RO.Close()
}

// RuntimeRegistry is the hub's concurrency-safe map from cache key to
// ProjectRuntime. Runtimes are created lazily on first access (design.md
// Decision 3: no upfront O(N) load, no eviction) and all stopped on server
// shutdown.
type RuntimeRegistry struct {
	ctx       context.Context
	cacheRoot string

	mu       sync.Mutex
	runtimes map[string]*ProjectRuntime
	closed   bool
}

// NewRuntimeRegistry returns an empty registry whose runtimes live under
// ctx: canceling ctx (server shutdown) stops every poller; Shutdown then
// waits for them and closes their stores.
func NewRuntimeRegistry(ctx context.Context, cacheRoot string) *RuntimeRegistry {
	return &RuntimeRegistry{ctx: ctx, cacheRoot: cacheRoot, runtimes: map[string]*ProjectRuntime{}}
}

// Get returns the runtime for key, creating it on first access. Unknown
// keys (no discovered database) and unreadable databases are rejected; the
// caller maps those to 404/500 responses.
func (r *RuntimeRegistry) Get(key string) (*ProjectRuntime, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, ErrRegistryClosed
	}

	if rt, ok := r.runtimes[key]; ok {
		return rt, nil
	}

	// Discovery is the source of truth for which keys exist; it's cheap
	// (metadata reads only) and never opens a runtime.
	projects, err := store.DiscoverProjectsWith(r.cacheRoot)
	if err != nil {
		return nil, err
	}
	var info *store.ProjectInfo
	for i := range projects {
		if projects[i].Key == key {
			info = &projects[i]
			break
		}
	}
	if info == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownProject, key)
	}
	if info.Status == store.StatusUnreadable {
		return nil, fmt.Errorf("%w: %s", ErrProjectUnreadable, key)
	}

	ro, err := store.OpenReadOnly(info.DBPath)
	if err != nil {
		return nil, fmt.Errorf("server: opening project %s: %w", key, err)
	}
	gs := NewGraphService()
	bc := NewBroadcaster()
	rctx, cancel := context.WithCancel(r.ctx)
	poller := NewPoller(ro, info.RepoPath, gs, bc.Notify)
	done := make(chan struct{})
	go func() {
		defer close(done)
		poller.Run(rctx)
	}()

	rt := &ProjectRuntime{Info: *info, RO: ro, GS: gs, BC: bc, cancel: cancel, done: done}
	r.runtimes[key] = rt
	return rt, nil
}

// Len reports how many runtimes are currently alive — used by tests to
// prove lazy loading (e.g. /api/projects never opens one).
func (r *RuntimeRegistry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.runtimes)
}

// Shutdown stops every runtime: cancels pollers, waits for their goroutines
// to exit, and closes their read-only stores.
func (r *RuntimeRegistry) Shutdown() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	runtimes := make([]*ProjectRuntime, 0, len(r.runtimes))
	for _, rt := range r.runtimes {
		runtimes = append(runtimes, rt)
	}
	r.runtimes = map[string]*ProjectRuntime{}
	r.mu.Unlock()

	for _, rt := range runtimes {
		rt.stop()
	}
}
