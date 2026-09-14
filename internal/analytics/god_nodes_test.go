package analytics

import (
	"fmt"
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
)

// starGraph builds a hub node connected to n leaves, so the hub has the
// highest degree by a wide margin.
func starGraph(t *testing.T, n int) *graph.Graph {
	t.Helper()
	g := graph.New()
	g.AddNode(&graph.Node{ID: "hub", Type: graph.NodeTypePackage})
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("leaf%d", i)
		g.AddNode(&graph.Node{ID: id, Type: graph.NodeTypePackage})
		must(t, g.AddEdge(&graph.Edge{ID: "e" + id, Type: graph.EdgeTypeImports, SrcID: "hub", DstID: id}))
	}
	return g
}

func TestDetectGodNodesMarksTopN(t *testing.T) {
	g := starGraph(t, 20)
	ComputeDegree(g)

	DetectGodNodes(g, 1)

	if !g.Node("hub").IsGodNode() {
		t.Error("expected hub (highest degree) to be marked a god node")
	}
	godNodes := g.GodNodes(-1)
	if len(godNodes) != 1 || godNodes[0].ID != "hub" {
		t.Errorf("expected exactly [hub] from GodNodes(-1), got %v", nodeIDs(godNodes))
	}
	for _, n := range g.Nodes() {
		if n.ID != "hub" && n.IsGodNode() {
			t.Errorf("expected leaf %s not to be marked a god node", n.ID)
		}
	}
}

func TestDetectGodNodesConfigurableCount(t *testing.T) {
	g := starGraph(t, 20)
	ComputeDegree(g)
	DetectGodNodes(g, 5)

	count := 0
	for _, n := range g.Nodes() {
		if n.IsGodNode() {
			count++
		}
	}
	if count != 5 {
		t.Errorf("expected 5 god nodes, got %d", count)
	}
}

func nodeIDs(nodes []*graph.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.ID
	}
	return out
}
