package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"kgraph/internal/analytics"
	"kgraph/internal/enrich"
	"kgraph/internal/graph"
	"kgraph/internal/store"
)

func testDeps(t *testing.T) Deps {
	t.Helper()
	g := graph.New()
	g.AddNode(&graph.Node{ID: "a", Type: graph.NodeTypeFunction, Hash: "h1", Signature: "func Alpha()", Properties: map[string]any{"name": "Alpha"}})
	g.AddNode(&graph.Node{ID: "b", Type: graph.NodeTypeFunction, Hash: "h2", Signature: "func Beta()", Properties: map[string]any{"name": "Beta"}})
	if err := g.AddEdge(&graph.Edge{ID: "e1", Type: graph.EdgeTypeCalls, SrcID: "a", DstID: "b"}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}
	enrich.AnnotateConfidence(g)
	if err := analytics.AnalyzeGraph(g, 1, analytics.DefaultResolution); err != nil {
		t.Fatalf("AnalyzeGraph() error = %v", err)
	}

	s, err := store.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })

	return Deps{Graph: g, Store: s}
}

func callToolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("expected exactly one content item, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

func TestQueryGraphHandlerReturnsSubgraph(t *testing.T) {
	deps := testDeps(t)
	res, err := queryGraphHandler(deps)(context.Background(), callToolRequest(map[string]any{"question": "Alpha"}))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), "Alpha") {
		t.Errorf("expected output to mention node 'a' (Alpha), got:\n%s", resultText(t, res))
	}
}

func TestQueryGraphHandlerRequiresQuestion(t *testing.T) {
	deps := testDeps(t)
	res, err := queryGraphHandler(deps)(context.Background(), callToolRequest(map[string]any{}))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result when question is missing")
	}
}

func TestGetNodeHandlerReturnsNodeDetails(t *testing.T) {
	deps := testDeps(t)
	res, err := getNodeHandler(deps)(context.Background(), callToolRequest(map[string]any{"id": "a"}))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", resultText(t, res))
	}
	if !strings.HasPrefix(resultText(t, res), "# a ") {
		t.Errorf("expected explain output for node 'a', got:\n%s", resultText(t, res))
	}
}

func TestGetNodeHandlerUnknownNode(t *testing.T) {
	deps := testDeps(t)
	res, err := getNodeHandler(deps)(context.Background(), callToolRequest(map[string]any{"id": "does-not-exist"}))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result for an unknown node")
	}
}

func TestShortestPathHandlerReturnsHops(t *testing.T) {
	deps := testDeps(t)
	res, err := shortestPathHandler(deps)(context.Background(), callToolRequest(map[string]any{"source": "a", "target": "b"}))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", resultText(t, res))
	}

	var hops []pathHopJSON
	if err := json.Unmarshal([]byte(resultText(t, res)), &hops); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, resultText(t, res))
	}
	if len(hops) != 2 {
		t.Fatalf("expected 2 hops (a, b), got %d: %+v", len(hops), hops)
	}
	if hops[0].NodeID != "a" || hops[1].NodeID != "b" {
		t.Errorf("expected hops a -> b, got %+v", hops)
	}
	if hops[1].EdgeType != string(graph.EdgeTypeCalls) {
		t.Errorf("expected edge type %q, got %q", graph.EdgeTypeCalls, hops[1].EdgeType)
	}
}

func TestShortestPathHandlerNoPath(t *testing.T) {
	deps := testDeps(t)
	deps.Graph.AddNode(&graph.Node{ID: "c", Type: graph.NodeTypePackage, Hash: "h3"})

	res, err := shortestPathHandler(deps)(context.Background(), callToolRequest(map[string]any{"source": "a", "target": "c"}))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error result when no path exists")
	}
}

func TestExportGraphHandlerReturnsFullGraph(t *testing.T) {
	deps := testDeps(t)
	res, err := exportGraphHandler(deps)(context.Background(), callToolRequest(nil))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", resultText(t, res))
	}

	var doc struct {
		NodeCount int `json:"node_count"`
		EdgeCount int `json:"edge_count"`
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, resultText(t, res))
	}
	if doc.NodeCount != 2 || doc.EdgeCount != 1 {
		t.Fatalf("expected node_count=2 edge_count=1, got %d/%d", doc.NodeCount, doc.EdgeCount)
	}
}

func TestNewServerRegistersAllTools(t *testing.T) {
	deps := testDeps(t)
	srv := NewServer(deps)
	if srv == nil {
		t.Fatal("NewServer() returned nil")
	}
}

func TestRequireBearerRejectsMissingOrWrongKey(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requireBearer("secret", inner)

	for name, headerVal := range map[string]string{
		"missing header": "",
		"wrong key":      "Bearer wrong",
	} {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if headerVal != "" {
			req.Header.Set("Authorization", headerVal)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", name, rec.Code)
		}
	}
}

func TestRequireBearerAcceptsCorrectKey(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requireBearer("secret", inner)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with correct key, got %d", rec.Code)
	}
}
