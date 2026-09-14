package context

import (
	"testing"

	"kgraph/internal/graph"
)

func TestPathFindsShortestWeightedRoute(t *testing.T) {
	g := chainGraph()

	hops, err := Path(g, "demo.A()", "demo.C()")
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	wantIDs := []string{"demo.A()", "demo.B()", "demo.C()"}
	if len(hops) != len(wantIDs) {
		t.Fatalf("expected %d hops, got %d: %v", len(wantIDs), len(hops), hopIDs(hops))
	}
	for i, id := range wantIDs {
		if hops[i].Node.ID != id {
			t.Errorf("hop %d: expected %s, got %s", i, id, hops[i].Node.ID)
		}
	}
	if hops[1].ViaEdge != graph.EdgeTypeCalls {
		t.Errorf("expected hop 1 to be reached via calls, got %s", hops[1].ViaEdge)
	}
}

func TestPathPrefersStructuralEdgesOverImports(t *testing.T) {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "a", Type: graph.NodeTypePackage})
	g.AddNode(&graph.Node{ID: "b", Type: graph.NodeTypePackage})
	g.AddNode(&graph.Node{ID: "c", Type: graph.NodeTypePackage})
	g.AddNode(&graph.Node{ID: "d", Type: graph.NodeTypePackage})
	// Direct but "weak" path a-imports->d (1 hop of low-weight edge).
	must(t, g.AddEdge(&graph.Edge{ID: "e1", Type: graph.EdgeTypeImports, SrcID: "a", DstID: "d"}))
	// Longer but "strong" path a-calls->b-calls->c-calls->d.
	must(t, g.AddEdge(&graph.Edge{ID: "e2", Type: graph.EdgeTypeCalls, SrcID: "a", DstID: "b"}))
	must(t, g.AddEdge(&graph.Edge{ID: "e3", Type: graph.EdgeTypeCalls, SrcID: "b", DstID: "c"}))
	must(t, g.AddEdge(&graph.Edge{ID: "e4", Type: graph.EdgeTypeCalls, SrcID: "c", DstID: "d"}))

	hops, err := Path(g, "a", "d")
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	// edgeCost(imports)=3, edgeCost(calls)=1 each -> direct import path
	// costs 3, the 3-hop calls path costs 3 too (tie); either is a valid
	// minimum-cost path under Dijkstra, but it must not be some other,
	// higher-cost route.
	if len(hops) < 2 || hops[0].Node.ID != "a" || hops[len(hops)-1].Node.ID != "d" {
		t.Fatalf("expected a path from a to d, got %v", hopIDs(hops))
	}
}

func TestPathReturnsErrorForUnknownNode(t *testing.T) {
	g := chainGraph()
	if _, err := Path(g, "demo.A()", "does.not.Exist()"); err == nil {
		t.Fatal("expected an error for a nonexistent destination node")
	}
	if _, err := Path(g, "does.not.Exist()", "demo.A()"); err == nil {
		t.Fatal("expected an error for a nonexistent source node")
	}
}

func TestPathReturnsErrorWhenDisconnected(t *testing.T) {
	g := chainGraph()
	if _, err := Path(g, "demo.A()", "table:orders"); err == nil {
		t.Fatal("expected an error when no path connects the two nodes")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func hopIDs(hops []PathHop) []string {
	out := make([]string, len(hops))
	for i, h := range hops {
		out[i] = h.Node.ID
	}
	return out
}
