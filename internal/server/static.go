package server

import (
	"net/http"

	"github.com/josinaldojr/kgraph/internal/viewer"
)

// staticAssetHandler serves app.css and app.js under /static/. Shared by
// both serving modes.
func staticAssetHandler() http.Handler {
	return http.StripPrefix("/static/", http.FileServerFS(viewer.Assets()))
}

// renderIndex executes internal/viewer's index template with cfg injected.
func renderIndex(w http.ResponseWriter, cfg viewer.BootstrapConfig) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := viewer.RenderIndex(w, cfg); err != nil {
		// Headers may already be flushed mid-body; best-effort log-free
		// drop matches writeJSON's stance on disconnected clients.
		_ = err
	}
}

// singleIndexHandler serves the single-project mode's exact pre-hub surface:
// the viewer page at / plus the static assets — now with the bootstrap
// config injected so app.js can stay mode-agnostic.
func singleIndexHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", staticAssetHandler())
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		renderIndex(w, viewer.BootstrapConfig{
			Mode:       "single",
			Page:       "viewer",
			APIBase:    "/api",
			EventsPath: "/events",
		})
	})
	return mux
}
