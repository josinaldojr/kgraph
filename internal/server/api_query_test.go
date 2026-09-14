package server

import (
	"net/http"
	"testing"
)

func TestHandleQueryReturnsRenderedResult(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var dto QueryDTO
	getJSON(t, srv.URL+"/api/query?q=invoice", &dto)
	if dto.Question != "invoice" {
		t.Errorf("expected question echoed back, got %q", dto.Question)
	}
	if dto.Result == "" {
		t.Error("expected a non-empty rendered result")
	}
}

func TestHandleQueryMissingParam(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/query")
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing q, got %d", res.StatusCode)
	}
}

func TestHandlePathFindsRoute(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var dto PathDTO
	getJSON(t, srv.URL+"/api/path?src=demo.Foo%28%29&dst=demo.Bar%28%29", &dto)
	if !dto.Found {
		t.Fatal("expected a path to be found between demo.Foo() and demo.Bar()")
	}
	if len(dto.Hops) != 2 || dto.Hops[0].NodeID != "demo.Foo()" || dto.Hops[1].NodeID != "demo.Bar()" {
		t.Errorf("unexpected hops: %+v", dto.Hops)
	}
}

func TestHandlePathNoRoute(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var dto PathDTO
	getJSON(t, srv.URL+"/api/path?src=demo.Foo%28%29&dst=demo.Widget", &dto)
	if dto.Found {
		t.Errorf("expected no path between disconnected nodes, got %+v", dto.Hops)
	}
}

func TestHandleExplainReturnsFullDetail(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	var dto ExplainDTO
	getJSON(t, srv.URL+"/api/explain?id=demo.Foo%28%29", &dto)
	if dto.NodeID != "demo.Foo()" {
		t.Errorf("expected node_id echoed back, got %q", dto.NodeID)
	}
	if dto.Result == "" {
		t.Error("expected a non-empty rendered explain result")
	}
}

func TestHandleExplainUnknownNode(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/explain?id=nope")
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown node, got %d", res.StatusCode)
	}
}

func TestHandleGraphIncludesEdgeConfidence(t *testing.T) {
	snap := buildAPIFixture()
	for _, e := range snap.Graph.Edges() {
		e.Confidence = "EXTRACTED"
	}
	srv := newTestServer(snap)
	defer srv.Close()

	var dto GraphDTO
	getJSON(t, srv.URL+"/api/graph?types=all", &dto)
	for _, e := range dto.Edges {
		if e.Confidence != "EXTRACTED" {
			t.Errorf("expected edge %+v to carry confidence, got %q", e, e.Confidence)
		}
	}
}

func TestHandleNodeIncludesAnalyticsFields(t *testing.T) {
	snap := buildAPIFixture()
	foo := snap.Graph.Node("demo.Foo()")
	foo.Properties = map[string]any{"degree": 3, "god_node": true, "community": 2, "community_label": "Demo"}
	srv := newTestServer(snap)
	defer srv.Close()

	var dto NodeDetailDTO
	getJSON(t, srv.URL+"/api/node?id=demo.Foo%28%29", &dto)
	if dto.Degree != 3 || !dto.GodNode || dto.Community != 2 || dto.CommunityLabel != "Demo" {
		t.Errorf("expected analytics fields in node detail, got %+v", dto)
	}
}

func TestHandlersNewReturn503BeforeFirstSnapshot(t *testing.T) {
	srv := newTestServer(nil)
	defer srv.Close()

	for _, path := range []string{"/api/query?q=x", "/api/path?src=a&dst=b", "/api/explain?id=x"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if res.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("%s: expected 503 before first snapshot, got %d", path, res.StatusCode)
		}
	}
}
