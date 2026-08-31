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
