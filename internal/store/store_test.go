package store

import (
	"path/filepath"
	"testing"

	"kgraph/internal/graph"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "graph.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleGraph() *graph.Graph {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "pkg:demo", Type: graph.NodeTypePackage, Hash: "h1", Properties: map[string]any{"name": "demo"}})
	g.AddNode(&graph.Node{ID: "demo.Foo()", Type: graph.NodeTypeFunction, Hash: "h2", Signature: "func Foo()"})
	g.AddEdge(&graph.Edge{ID: "e1", Type: graph.EdgeTypeImports, SrcID: "pkg:demo", DstID: "demo.Foo()"})
	return g
}

func TestSaveAndLoadGraphRoundTrip(t *testing.T) {
	s := openTestStore(t)
	g := sampleGraph()

	if _, err := s.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph() error = %v", err)
	}

	loaded, err := s.LoadGraph()
	if err != nil {
		t.Fatalf("LoadGraph() error = %v", err)
	}
	if loaded.NodeCount() != g.NodeCount() || loaded.EdgeCount() != g.EdgeCount() {
		t.Fatalf("round-trip mismatch: got %d nodes/%d edges, want %d/%d",
			loaded.NodeCount(), loaded.EdgeCount(), g.NodeCount(), g.EdgeCount())
	}
	fn := loaded.Node("demo.Foo()")
	if fn == nil || fn.Signature != "func Foo()" {
		t.Fatalf("expected loaded function node with signature preserved, got %+v", fn)
	}
}

func TestSaveAndLoadGraphPreservesEdgeConfidence(t *testing.T) {
	s := openTestStore(t)
	g := graph.New()
	g.AddNode(&graph.Node{ID: "demo.Foo()", Type: graph.NodeTypeFunction, Hash: "h1"})
	g.AddNode(&graph.Node{ID: "table:orders", Type: graph.NodeTypeTable, Hash: "h2"})
	if err := g.AddEdge(&graph.Edge{ID: "e1", Type: graph.EdgeTypeReadsTable, SrcID: "demo.Foo()", DstID: "table:orders", Confidence: graph.ConfidenceInferred}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}

	if _, err := s.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph() error = %v", err)
	}

	loaded, err := s.LoadGraph()
	if err != nil {
		t.Fatalf("LoadGraph() error = %v", err)
	}
	edges := loaded.OutEdges("demo.Foo()")
	if len(edges) != 1 {
		t.Fatalf("expected 1 loaded edge, got %d", len(edges))
	}
	if edges[0].Confidence != graph.ConfidenceInferred {
		t.Errorf("expected confidence %q to round-trip unchanged, got %q", graph.ConfidenceInferred, edges[0].Confidence)
	}
}

func TestSaveGraphIsIdempotentByHash(t *testing.T) {
	s := openTestStore(t)
	g := sampleGraph()

	first, err := s.SaveGraph(g)
	if err != nil {
		t.Fatalf("first SaveGraph() error = %v", err)
	}
	if first.NodesWritten != 2 {
		t.Fatalf("expected 2 nodes written on first save, got %d", first.NodesWritten)
	}

	second, err := s.SaveGraph(g)
	if err != nil {
		t.Fatalf("second SaveGraph() error = %v", err)
	}
	if second.NodesWritten != 0 || second.NodesUnchanged != 2 {
		t.Errorf("expected a no-op second save (0 written, 2 unchanged), got written=%d unchanged=%d",
			second.NodesWritten, second.NodesUnchanged)
	}
}

func TestRefreshNodePropertiesBypassesHashGate(t *testing.T) {
	s := openTestStore(t)
	g := sampleGraph()
	if _, err := s.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph() error = %v", err)
	}

	// Mutate a node's Properties without changing its Hash — SaveGraph
	// alone must NOT persist this (graph-storage's content-hash gate), but
	// RefreshNodeProperties must.
	pkgNode := g.Node("pkg:demo")
	pkgNode.Properties["degree"] = 3

	stats, err := s.SaveGraph(g)
	if err != nil {
		t.Fatalf("second SaveGraph() error = %v", err)
	}
	if stats.NodesWritten != 0 {
		t.Fatalf("expected SaveGraph to skip the hash-unchanged node, got %d writes", stats.NodesWritten)
	}

	loaded, err := s.LoadGraph()
	if err != nil {
		t.Fatalf("LoadGraph() error = %v", err)
	}
	if _, ok := loaded.Node("pkg:demo").Properties["degree"]; ok {
		t.Fatal("expected SaveGraph alone not to persist the property change on a hash-unchanged node")
	}

	if err := s.RefreshNodeProperties(g); err != nil {
		t.Fatalf("RefreshNodeProperties() error = %v", err)
	}
	loaded, err = s.LoadGraph()
	if err != nil {
		t.Fatalf("LoadGraph() error = %v", err)
	}
	if got := loaded.Node("pkg:demo").Degree(); got != 3 {
		t.Errorf("expected RefreshNodeProperties to persist degree=3, got %d", got)
	}
}

func TestBuildMetaRoundTrip(t *testing.T) {
	s := openTestStore(t)

	if _, ok, err := s.LastCommit("/repo"); err != nil || ok {
		t.Fatalf("expected no prior commit, got ok=%v err=%v", ok, err)
	}
	if err := s.SetLastCommit("/repo", "abc123"); err != nil {
		t.Fatalf("SetLastCommit() error = %v", err)
	}
	commit, ok, err := s.LastCommit("/repo")
	if err != nil || !ok || commit != "abc123" {
		t.Fatalf("expected commit=abc123 ok=true, got commit=%q ok=%v err=%v", commit, ok, err)
	}
}
