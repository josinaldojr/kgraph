package server

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"

	"kgraph/internal/store"
)

// ProjectDTO is one discovered project as returned by GET /api/projects and
// rendered by the hub picker.
type ProjectDTO struct {
	Key         string `json:"key"`
	Name        string `json:"name"` // display name: repo basename, or key when unknown
	RepoPath    string `json:"repo_path,omitempty"`
	Status      string `json:"status"`    // built | never-built | unreadable
	Freshness   string `json:"freshness"` // current | stale | missing | unknown
	NodeCount   int    `json:"node_count"`
	EdgeCount   int    `json:"edge_count"`
	LastBuildAt int64  `json:"last_build_at,omitempty"`
}

func toProjectDTO(p store.ProjectInfo, fresh store.Freshness) ProjectDTO {
	name := p.Key
	if p.RepoPath != "" {
		name = filepath.Base(p.RepoPath)
	}
	return ProjectDTO{
		Key:         p.Key,
		Name:        name,
		RepoPath:    p.RepoPath,
		Status:      string(p.Status),
		Freshness:   string(fresh),
		NodeCount:   p.NodeCount,
		EdgeCount:   p.EdgeCount,
		LastBuildAt: p.LastBuildAt,
	}
}

// Hub is the multi-project serving mode: it discovers every built graph in
// the cache, serves the picker page at /, and lazily serves each project's
// viewer + API under /p/<key>/ and /api/projects/<key>/ — design.md
// Decisions 3 and 4.
type Hub struct {
	cacheRoot string
	registry  *RuntimeRegistry
}

// NewHub returns a Hub whose runtimes live under ctx; call Shutdown after
// the HTTP server stops.
func NewHub(ctx context.Context, cacheRoot string) *Hub {
	return &Hub{cacheRoot: cacheRoot, registry: NewRuntimeRegistry(ctx, cacheRoot)}
}

// Shutdown stops every per-project runtime created during the hub's life.
func (h *Hub) Shutdown() { h.registry.Shutdown() }

// Registry exposes the runtime registry for tests (lazy-loading assertions).
func (h *Hub) Registry() *RuntimeRegistry { return h.registry }

// Handler returns the hub's http.Handler.
func (h *Hub) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.handlePickerPage)
	mux.HandleFunc("GET /api/projects", h.handleProjects)
	mux.HandleFunc("GET /p/{key}", h.handleViewerRedirect)
	mux.HandleFunc("GET /p/{key}/{$}", h.handleViewerPage)
	mux.HandleFunc("GET /api/projects/{key}/graph", h.projectHandler(apiGraph))
	mux.HandleFunc("GET /api/projects/{key}/graph/local", h.projectHandler(apiGraphLocal))
	mux.HandleFunc("GET /api/projects/{key}/node", h.projectHandler(apiNode))
	mux.HandleFunc("GET /api/projects/{key}/search", h.projectHandler(apiSearch))
	mux.HandleFunc("GET /api/projects/{key}/events", h.handleProjectEvents)
	mux.Handle("/static/", staticAssetHandler())
	return mux
}

// handlePickerPage serves the project-selection view at the hub root — no
// graph canvas, per graph-visualization's "Project selection on hub
// startup" requirement.
func (h *Hub) handlePickerPage(w http.ResponseWriter, r *http.Request) {
	renderIndex(w, bootstrapConfig{Mode: "hub", Page: "picker", APIBase: "/api"})
}

// handleViewerRedirect maps /p/<key> to /p/<key>/ so direct links work with
// or without the trailing slash.
func (h *Hub) handleViewerRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
}

// handleViewerPage serves a project's viewer page under its own linkable
// route, loading nothing until the runtime registry actually resolves the
// project (which itself loads on demand).
func (h *Hub) handleViewerPage(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if _, err := h.registry.Get(key); err != nil {
		writeProjectError(w, key, err)
		return
	}
	renderIndex(w, bootstrapConfig{
		Mode:       "hub",
		Page:       "viewer",
		ProjectKey: key,
		APIBase:    "/api/projects/" + key,
		EventsPath: "/api/projects/" + key + "/events",
		HomeURL:    "/",
	})
}

// handleProjects serves GET /api/projects: a discovery scan plus freshness,
// as JSON. It never opens a runtime — the picker lists projects without
// loading any graph (design.md Decision 3).
func (h *Hub) handleProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := store.DiscoverProjectsWith(h.cacheRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dtos := make([]ProjectDTO, 0, len(projects))
	for _, p := range projects {
		dtos = append(dtos, toProjectDTO(p, store.ProjectFreshness(p)))
	}
	writeJSON(w, dtos)
}

// handleProjectEvents serves GET /api/projects/<key>/events — that project's
// own SSE stream. Isolation comes from each runtime owning its own
// Broadcaster: a rebuild elsewhere notifies a different broadcaster and
// this stream is never woken.
func (h *Hub) handleProjectEvents(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	rt, err := h.registry.Get(key)
	if err != nil {
		writeProjectError(w, key, err)
		return
	}
	streamEvents(w, r, rt.BC)
}

// projectHandler adapts a shared snapshot-parameterized API handler to a
// hub route: resolve the runtime for the {key} path segment, then the
// snapshot, then run the handler — the same implementation single-project
// mode uses, per design.md Decision 4.
func (h *Hub) projectHandler(fn func(http.ResponseWriter, *http.Request, *Snapshot)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		rt, err := h.registry.Get(key)
		if err != nil {
			writeProjectError(w, key, err)
			return
		}
		snap := rt.GS.Snapshot()
		if snap == nil {
			writeError(w, http.StatusServiceUnavailable, "graph not loaded yet")
			return
		}
		fn(w, r, snap)
	}
}

// writeProjectError maps registry errors to client responses: unknown keys
// get a clear JSON 404 (never an empty or misleading graph), unreadable
// databases a clear 500, everything else a generic 500.
func writeProjectError(w http.ResponseWriter, key string, err error) {
	switch {
	case errors.Is(err, ErrUnknownProject):
		writeError(w, http.StatusNotFound, "unknown project: "+key)
	case errors.Is(err, ErrProjectUnreadable):
		writeError(w, http.StatusInternalServerError, "project database is unreadable: "+key)
	case errors.Is(err, ErrRegistryClosed):
		writeError(w, http.StatusServiceUnavailable, "server is shutting down")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
