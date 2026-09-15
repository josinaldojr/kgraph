package export

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestToHTMLProducesValidHTML(t *testing.T) {
	g := testGraph(t)
	s := testStore(t)

	out, err := ToHTML(g, s, "/repos/demo")
	if err != nil {
		t.Fatalf("ToHTML() error = %v", err)
	}
	if !bytes.Contains(out, []byte("<!doctype html>")) {
		t.Errorf("expected output to start with a doctype, got:\n%s", out)
	}
	if !bytes.Contains(out, []byte(`id="graph"`)) {
		t.Errorf("expected output to contain the graph canvas markup, got:\n%s", out)
	}
}

func TestToHTMLEmbedsSameDatasetAsToJSON(t *testing.T) {
	g := testGraph(t)
	s := testStore(t)

	jsonOut, err := ToJSON(g, s, "/repos/demo")
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}
	var wantDoc Graph
	if err := json.Unmarshal(jsonOut, &wantDoc); err != nil {
		t.Fatalf("ToJSON() output is not valid JSON: %v", err)
	}

	htmlOut, err := ToHTML(g, s, "/repos/demo")
	if err != nil {
		t.Fatalf("ToHTML() error = %v", err)
	}
	bootstrap := extractBootstrapJSON(t, string(htmlOut))

	var embedded struct {
		Data Graph `json:"data"`
	}
	if err := json.Unmarshal([]byte(bootstrap), &embedded); err != nil {
		t.Fatalf("window.KGRAPH is not valid JSON: %v\n%s", err, bootstrap)
	}
	if !reflect.DeepEqual(wantDoc, embedded.Data) {
		t.Errorf("expected window.KGRAPH.data to equal ToJSON's document, got:\n%+v\nwant:\n%+v", embedded.Data, wantDoc)
	}
}

// extractBootstrapJSON pulls the JSON object assigned to window.KGRAPH out
// of a rendered index.html page (`<script>window.KGRAPH = {...};</script>`).
func extractBootstrapJSON(t *testing.T, html string) string {
	t.Helper()
	const marker = "window.KGRAPH = "
	start := strings.Index(html, marker)
	if start == -1 {
		t.Fatalf("expected %q in rendered HTML", marker)
	}
	start += len(marker)
	end := strings.Index(html[start:], ";</script>")
	if end == -1 {
		t.Fatalf("expected a terminating \";</script>\" after window.KGRAPH assignment")
	}
	return html[start : start+end]
}

func TestToHTMLSetsStaticBootstrapFlag(t *testing.T) {
	g := testGraph(t)
	s := testStore(t)

	out, err := ToHTML(g, s, "/repos/demo")
	if err != nil {
		t.Fatalf("ToHTML() error = %v", err)
	}
	if !bytes.Contains(out, []byte(`"static":true`)) {
		t.Errorf("expected window.KGRAPH.static to be true, got:\n%s", out)
	}
}

func TestToHTMLHasNoExternalReferences(t *testing.T) {
	g := testGraph(t)
	s := testStore(t)

	out, err := ToHTML(g, s, "/repos/demo")
	if err != nil {
		t.Fatalf("ToHTML() error = %v", err)
	}
	// The generated markup/config must reference no external URL a browser
	// would actually load: no linked/sourced static asset, and no apiBase
	// pointing at a live server (bare mentions inside app.js's own doc
	// comments, e.g. "the same shape /api/graph does", are fine — they're
	// never fetched).
	for _, forbidden := range []string{`href="/static`, `src="/static`, `"apiBase":"`} {
		if bytes.Contains(out, []byte(forbidden)) {
			t.Errorf("expected graph.html to contain no reference to %q, but it does", forbidden)
		}
	}
	if !bytes.Contains(out, []byte("<style>")) {
		t.Error("expected app.css to be inlined into a <style> block")
	}
	if !bytes.Contains(out, []byte("function loadGlobalData")) {
		t.Error("expected app.js to be inlined into a <script> block")
	}
}
