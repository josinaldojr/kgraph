package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/store"
)

// buildAPIFixture assembles a small snapshot: a package with two functions
// (one summarized, one stale), a struct/table pair, and a Field node that
// must never surface as its own summarizable entity.
func buildAPIFixture() *Snapshot {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "pkg:demo", Type: graph.NodeTypePackage, Hash: "h1"})
	g.AddNode(&graph.Node{ID: "demo.Foo()", Type: graph.NodeTypeFunction, Signature: "func Foo()", File: "demo.go", LineStart: 3, LineEnd: 5, Hash: "h2"})
	g.AddNode(&graph.Node{ID: "demo.Bar()", Type: graph.NodeTypeFunction, Signature: "func Bar()", File: "demo.go", LineStart: 7, LineEnd: 9, Hash: "h3"})
	g.AddNode(&graph.Node{ID: "demo.Widget", Type: graph.NodeTypeStruct, File: "demo.go", Hash: "h4"})
	g.AddNode(&graph.Node{ID: graph.FieldID("demo.Widget", "Name"), Type: graph.NodeTypeField, Hash: "h5"})
	_ = g.AddEdge(&graph.Edge{ID: "e1", Type: graph.EdgeTypeCalls, SrcID: "demo.Foo()", DstID: "demo.Bar()"})
	_ = g.AddEdge(&graph.Edge{ID: "e2", Type: graph.EdgeTypeHasField, SrcID: "demo.Widget", DstID: graph.FieldID("demo.Widget", "Name")})

	return &Snapshot{
		Graph: g,
		NodeSummaries: map[string]store.SummaryRecord{
			"demo.Foo()": {NodeID: "demo.Foo()", Level: "node", Hash: "h2", Summary: "Formats an invoice."},
			"demo.Bar()": {NodeID: "demo.Bar()", Level: "node", Hash: "h3", Summary: "Old summary.", Stale: true},
		},
		FileSummaries:   map[string]store.SummaryRecord{"demo.go": {NodeID: "demo.go", Level: "file", Summary: "Declares Foo and Bar."}},
		ModuleSummaries: map[string]store.SummaryRecord{},
	}
}

func newTestServer(snap *Snapshot) *httptest.Server {
	gs := NewGraphService()
	if snap != nil {
		gs.Store(snap)
	}
	s := New(gs, NewBroadcaster())
	return httptest.NewServer(s.Handler())
}

func getJSON(t *testing.T, url string, v any) *http.Response {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	if v != nil {
		defer res.Body.Close()
		if err := json.NewDecoder(res.Body).Decode(v); err != nil {
			t.Fatalf("decoding response from %s: %v", url, err)
		}
	}
	return res
}

func TestHandleGraphDefaultFiltersHideFieldAndColumn(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var dto GraphDTO
	getJSON(t, srv.URL+"/api/graph", &dto)

	for _, n := range dto.Nodes {
		if n.Type == "Field" || n.Type == "Column" {
			t.Errorf("expected Field/Column hidden by default, got %+v", n)
		}
	}
	if len(dto.Nodes) != 4 { // pkg, Foo, Bar, Widget
		t.Errorf("expected 4 nodes with default filters, got %d: %+v", len(dto.Nodes), dto.Nodes)
	}
}

func TestHandleGraphTypesAllIncludesEverything(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var dto GraphDTO
	getJSON(t, srv.URL+"/api/graph?types=all", &dto)
	if len(dto.Nodes) != 5 {
		t.Errorf("expected 5 nodes with types=all, got %d", len(dto.Nodes))
	}
}

func TestHandleGraphLocalCentersOnNode(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var dto GraphDTO
	getJSON(t, srv.URL+"/api/graph/local?id=demo.Foo%28%29&hops=1", &dto)

	ids := map[string]bool{}
	for _, n := range dto.Nodes {
		ids[n.ID] = true
	}
	if !ids["demo.Foo()"] || !ids["demo.Bar()"] {
		t.Fatalf("expected local graph around demo.Foo() to include demo.Bar(), got %+v", dto.Nodes)
	}
}

func TestHandleGraphLocalUnknownNode(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	res := getJSON(t, srv.URL+"/api/graph/local?id=nope", nil)
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown node, got %d", res.StatusCode)
	}
}

func TestHandleNodeReportsNoteStates(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var foo NodeDetailDTO
	getJSON(t, srv.URL+"/api/node?id=demo.Foo%28%29", &foo)
	if foo.Note.State != "current" || foo.Note.Summary != "Formats an invoice." {
		t.Errorf("expected current note for Foo, got %+v", foo.Note)
	}
	if foo.FileNote == nil || foo.FileNote.Summary != "Declares Foo and Bar." {
		t.Errorf("expected Foo's file note to surface as a side fact, got %+v", foo.FileNote)
	}
	if len(foo.Relations) != 1 || foo.Relations[0].NodeID != "demo.Bar()" {
		t.Errorf("expected Foo's one outgoing relation to Bar, got %+v", foo.Relations)
	}

	var bar NodeDetailDTO
	getJSON(t, srv.URL+"/api/node?id=demo.Bar%28%29", &bar)
	if bar.Note.State != "stale" {
		t.Errorf("expected stale note state for Bar, got %+v", bar.Note)
	}

	var widget NodeDetailDTO
	getJSON(t, srv.URL+"/api/node?id=demo.Widget", &widget)
	if widget.Note.State != "none" {
		t.Errorf("expected no-note state for Widget, got %+v", widget.Note)
	}
}

func TestHandleSearchRanksBySummary(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var results []SearchResultDTO
	getJSON(t, srv.URL+"/api/search?q=invoice", &results)
	if len(results) == 0 || results[0].ID != "demo.Foo()" {
		t.Fatalf("expected demo.Foo() to rank first for 'invoice', got %+v", results)
	}
}

func TestHandlersReturn503BeforeFirstSnapshot(t *testing.T) {
	srv := newTestServer(nil)
	defer srv.Close()

	for _, path := range []string{"/api/graph", "/api/node?id=x", "/api/graph/local?id=x", "/api/search?q=x"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if res.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("%s: expected 503 before first snapshot, got %d", path, res.StatusCode)
		}
	}
}

func TestStaticIndexServed(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /, got %d", res.StatusCode)
	}
}
