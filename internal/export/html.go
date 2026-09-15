package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/store"
	"github.com/josinaldojr/kgraph/internal/viewer"
)

// ToHTML renders a single self-contained graph.html: the same force-directed
// viewer `kgraph serve` renders (internal/viewer), with app.css/app.js
// inlined and the full graph.json dataset (ToJSON's already-assembled
// document, reused directly) embedded as window.KGRAPH.data, so the file
// opens and is fully explorable straight from disk — no server, no
// database, no network request — per graph-html-export's requirements
// (design.md Decision 2).
func ToHTML(g *graph.Graph, s *store.Store, repoPath string) ([]byte, error) {
	data, err := ToJSON(g, s, repoPath)
	if err != nil {
		return nil, fmt.Errorf("export: building graph.html dataset: %w", err)
	}

	cfg := viewer.BootstrapConfig{
		Mode:   "static",
		Page:   "viewer",
		Static: true,
		Data:   json.RawMessage(data),
	}

	var buf bytes.Buffer
	if err := viewer.RenderIndex(&buf, cfg); err != nil {
		return nil, fmt.Errorf("export: rendering graph.html: %w", err)
	}

	return []byte(inlineAssets(buf.String())), nil
}

// inlineAssets replaces index.html's external stylesheet <link> and script
// <script src> tags with the viewer's actual CSS/JS content, so the result
// references nothing outside the file itself — design.md Decision 2.
func inlineAssets(html string) string {
	html = strings.Replace(html,
		`<link rel="stylesheet" href="/static/app.css">`,
		"<style>\n"+string(viewer.CSS())+"\n</style>",
		1)
	html = strings.Replace(html,
		`<script src="/static/app.js"></script>`,
		"<script>\n"+string(viewer.JS())+"\n</script>",
		1)
	return html
}
