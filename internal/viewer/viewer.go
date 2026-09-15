// Package viewer holds the kgraph web frontend's static assets
// (index.html, app.css, app.js: a vanilla-JS/canvas, no-build-step
// force-directed graph viewer) and the bootstrap-config/template-execution
// logic that injects a page's runtime configuration into index.html. It is
// shared by internal/server (which serves these assets live, over HTTP,
// against a database connection) and internal/export (which inlines them
// into a single self-contained graph.html with an embedded dataset) — see
// the html-export-and-modularity-communities change's design.md Decision 1.
package viewer

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"io"
	"io/fs"
	"strings"
	"sync"
)

//go:embed static/index.html static/app.css static/app.js
var staticFS embed.FS

// BootstrapConfig is the configuration injected into index.html as
// window.KGRAPH, letting one template/asset set serve every mode without a
// build step. The frontend builds its API URLs from APIBase and its SSE URL
// from EventsPath, so the same viewer code works unprefixed in
// single-project server mode, under /api/projects/<key>/ in hub mode, and
// — with Static set — entirely offline against an embedded Data payload in
// exported graph.html files.
type BootstrapConfig struct {
	Mode       string          `json:"mode"`                 // "hub" | "single" | "static"
	Page       string          `json:"page"`                 // "picker" | "viewer"
	ProjectKey string          `json:"projectKey,omitempty"` // set for hub-mode project pages
	APIBase    string          `json:"apiBase,omitempty"`    // "/api" or "/api/projects/<key>"; unused when Static
	EventsPath string          `json:"eventsPath,omitempty"` // "" when there's no live stream (hub picker, or Static)
	HomeURL    string          `json:"homeURL,omitempty"`    // "/" on hub-mode viewers: the way back to the picker
	Static     bool            `json:"static,omitempty"`     // true in exported graph.html: no server, use Data
	Data       json.RawMessage `json:"data,omitempty"`       // the full embedded graph dataset, only set when Static
}

// PageData is index.html's template context. Bootstrap carries the config
// as pre-serialized JSON (template.JS so html/template injects it verbatim
// into the <script> tag); Page/HomeURL drive the markup conditionals.
type PageData struct {
	Bootstrap template.JS
	Page      string
	HomeURL   string
}

var (
	indexTmplOnce sync.Once
	indexTmpl     *template.Template
)

// IndexTemplate parses the embedded index.html once. A failure here means
// the embed directive or the template itself is broken — a build-time bug.
func IndexTemplate() *template.Template {
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

// RenderIndex executes the index template with cfg injected, writing the
// result to w. The bootstrap JSON is encoded with HTML escaping so no
// repository path (or, in static mode, node/edge content) can break out of
// the <script> tag.
func RenderIndex(w io.Writer, cfg BootstrapConfig) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	data := PageData{
		Bootstrap: template.JS(strings.TrimSpace(buf.String())),
		Page:      cfg.Page,
		HomeURL:   cfg.HomeURL,
	}
	return IndexTemplate().Execute(w, data)
}

// Assets returns the static/ subtree (app.css, app.js) as its own
// filesystem, rooted so "app.css"/"app.js" are top-level — what
// internal/server's staticAssetHandler serves under /static/.
func Assets() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// staticFS is compiled in via go:embed; a failure here means the
		// embed directive itself is broken, which is a build-time bug.
		panic(err)
	}
	return sub
}

// CSS returns app.css's raw contents, for inlining into a self-contained
// export (design.md Decision 2).
func CSS() []byte {
	return mustRead("static/app.css")
}

// JS returns app.js's raw contents, for inlining into a self-contained
// export (design.md Decision 2).
func JS() []byte {
	return mustRead("static/app.js")
}

func mustRead(name string) []byte {
	b, err := staticFS.ReadFile(name)
	if err != nil {
		// staticFS is compiled in via go:embed; a failure here means the
		// embed directive itself is broken, which is a build-time bug.
		panic(err)
	}
	return b
}
