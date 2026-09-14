package analytics

import (
	"testing"

	"kgraph/internal/graph"
)

func TestComputeDegree(t *testing.T) {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "a", Type: graph.NodeTypePackage})
	g.AddNode(&graph.Node{ID: "b", Type: graph.NodeTypePackage})
	g.AddNode(&graph.Node{ID: "c", Type: graph.NodeTypePackage})
	must(t, g.AddEdge(&graph.Edge{ID: "e1", Type: graph.EdgeTypeImports, SrcID: "a", DstID: "b"}))
	must(t, g.AddEdge(&graph.Edge{ID: "e2", Type: graph.EdgeTypeImports, SrcID: "a", DstID: "c"}))

	ComputeDegree(g)

	if got := g.Node("a").Degree(); got != 2 {
		t.Errorf("expected a.Degree()=2, got %d", got)
	}
	if got := g.Node("b").Degree(); got != 1 {
		t.Errorf("expected b.Degree()=1, got %d", got)
	}
	if got := g.Node("c").Degree(); got != 1 {
		t.Errorf("expected c.Degree()=1, got %d", got)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
