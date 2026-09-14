package server

import (
	"log"
	"net/http"
)

// Server wires a GraphService and Broadcaster into an http.Handler serving
// the JSON API, the SSE stream, and the embedded frontend. Every handler
// here only ever reads from gs's current Snapshot — per
// graph-visualization's "Read-only against the graph store" requirement,
// nothing in this package holds a *store.Store or *store.ReadOnlyStore
// capable of a write; only cmd_serve.go's Poller does, and it never writes
// either (see internal/store.ReadOnlyStore).
type Server struct {
	gs *GraphService
	bc *Broadcaster
}

// New returns a Server backed by gs (kept fresh by a Poller — see
// poller.go) and bc (notified by that same Poller on each reload).
func New(gs *GraphService, bc *Broadcaster) *Server {
	return &Server{gs: gs, bc: bc}
}

// Handler returns the http.Handler `kgraph serve` listens with in
// single-project mode: the exact pre-hub surface (viewer at /, unprefixed
// API, /events), preserved byte-for-byte in routing by kgraph-project-hub's
// backward-compatibility requirement.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/graph", s.handleGraph)
	mux.HandleFunc("GET /api/graph/local", s.handleGraphLocal)
	mux.HandleFunc("GET /api/node", s.handleNode)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/query", s.handleQuery)
	mux.HandleFunc("GET /api/path", s.handlePath)
	mux.HandleFunc("GET /api/explain", s.handleExplain)
	mux.HandleFunc("GET /events", s.handleEvents)
	mux.Handle("/", singleIndexHandler())
	return recoverMiddleware(mux)
}

// recoverMiddleware isolates a panic in any handler to the request that
// triggered it: it logs the panic (method, path, recovered value) and
// responds 500, rather than letting the panic unwind past net/http's own
// per-connection recover and take the whole process down — significant in
// hub mode, where one process serves every discovered project (see
// server-api's "A handler panic SHALL NOT crash the server process"
// requirement).
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("server: recovered panic in %s %s: %v", r.Method, r.URL.Path, rec)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
