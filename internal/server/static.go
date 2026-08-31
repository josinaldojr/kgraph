package server

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"
)

//go:embed static/index.html static/app.css static/app.js
var staticFS embed.FS

// bootstrapConfig is the server-rendered configuration injected into
// index.html, letting one template serve every mode without a build step —
// design.md Decision 5. The frontend builds its API URLs from APIBase and
// its SSE URL from EventsPath, so the same viewer code works unprefixed in
// single-project mode and under /api/projects/<key>/ in hub mode.
type bootstrapConfig struct {
	Mode       string `json:"mode"`                 // server mode: "hub" | "single"
	Page       string `json:"page"`                 // "picker" | "viewer"
	ProjectKey string `json:"projectKey,omitempty"` // set for hub-mode project pages
	APIBase    string `json:"apiBase"`              // "/api" or "/api/projects/<key>"
	EventsPath string `json:"eventsPath,omitempty"` // "" on the hub picker (no stream there)
	HomeURL    string `json:"homeURL,omitempty"`    // "/" on hub-mode viewers: the way back to the picker
}

// pageData is index.html's template context. Bootstrap carries the config
// as pre-serialized JSON (template.JS so html/template injects it verbatim
// into the <script> tag); Hub/HomeURL drive the markup conditionals.
type pageData struct {
	Bootstrap template.JS
	Page      string
	HomeURL   string
}

var (
	indexTmplOnce sync.Once
	indexTmpl     *template.Template
)

// indexTemplate parses the embedded index.html once. A failure here means
// the embed directive or the template itself is broken — a build-time bug.
func indexTemplate() *template.Template {
	indexTmplOnce.Do(func() {
		b, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			panic(err)
		}
		indexTmpl, err = template.New("index.html").Parse(string(b))
		if err != nil {
			panic(err)
		}
	})
	return indexTmpl
}

// renderIndex executes the index template with cfg injected. The JSON is
// encoded with HTML escaping so no repository path can break out of the
// <script> tag.
func renderIndex(w http.ResponseWriter, cfg bootstrapConfig) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "encoding page config: "+err.Error())
		return
	}
	data := pageData{
		Bootstrap: template.JS(strings.TrimSpace(buf.String())),
		Page:      cfg.Page,
		HomeURL:   cfg.HomeURL,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate().Execute(w, data); err != nil {
		// Headers may already be flushed mid-body; best-effort log-free
		// drop matches writeJSON's stance on disconnected clients.
		_ = err
	}
}

// staticAssetHandler serves static/app.css and static/app.js under
// /static/. Shared by both serving modes.
func staticAssetHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// staticFS is compiled in via go:embed; a failure here means the
		// embed directive itself is broken, which is a build-time bug.
		panic(err)
	}
	return http.StripPrefix("/static/", http.FileServerFS(sub))
}

// singleIndexHandler serves the single-project mode's exact pre-hub surface:
// the viewer page at / plus the static assets — now with the bootstrap
// config injected so app.js can stay mode-agnostic.
func singleIndexHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", staticAssetHandler())
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		renderIndex(w, bootstrapConfig{
			Mode:       "single",
			Page:       "viewer",
			APIBase:    "/api",
			EventsPath: "/events",
		})
	})
	return mux
}
