package server

import (
	"fmt"
	"net/http"
)

// handleEvents serves GET /events — an SSE stream emitting a
// "graph-updated" event whenever the poller swaps in a new snapshot, per
// graph-visualization's "Live refresh on graph change" requirement.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	streamEvents(w, r, s.bc)
}

// streamEvents implements the SSE stream against a specific Broadcaster,
// shared by single-project mode (/events) and hub mode's per-project
// streams (/api/projects/<key>/events). Scoping the stream to one
// broadcaster is exactly what gives the hub its per-project live-refresh
// isolation: each project runtime owns its own Broadcaster, so a rebuild of
// project B can only wake project B's subscribers.
func streamEvents(w http.ResponseWriter, r *http.Request, bc *Broadcaster) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	ch, cancel := bc.Subscribe()
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			fmt.Fprint(w, "event: graph-updated\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}
