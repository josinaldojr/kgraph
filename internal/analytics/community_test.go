package analytics

import (
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
)

// twoClusterGraph builds two disjoint, densely-connected clusters (a1-a4
// and b1-b4), with no edges between them, so a correct community
// detector must put each cluster in its own community.
func twoClusterGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()
	clusterA := []string{"a1", "a2", "a3", "a4"}
	clusterB := []string{"b1", "b2", "b3", "b4"}
	for _, id := range append(append([]string{}, clusterA...), clusterB...) {
		g.AddNode(&graph.Node{ID: id, Type: graph.NodeTypeFunction, File: "pkg/" + id[:1] + "/x.go"})
	}
	connectAll := func(ids []string) {
		for i := range ids {
			for j := i + 1; j < len(ids); j++ {
				eid := ids[i] + "-" + ids[j]
				must(t, g.AddEdge(&graph.Edge{ID: eid, Type: graph.EdgeTypeCalls, SrcID: ids[i], DstID: ids[j]}))
			}
		}
	}
	connectAll(clusterA)
	connectAll(clusterB)
	return g
}

func TestDetectCommunitiesSeparatesDisjointClusters(t *testing.T) {
	g := twoClusterGraph(t)
	DetectCommunities(g, DefaultResolution)

	commA := g.Node("a1").Community()
	commB := g.Node("b1").Community()
	if commA == -1 || commB == -1 {
		t.Fatalf("expected both clusters to get a community, got a=%d b=%d", commA, commB)
	}
	if commA == commB {
		t.Fatalf("expected disjoint clusters to land in different communities, both got %d", commA)
	}
	for _, id := range []string{"a1", "a2", "a3", "a4"} {
		if got := g.Node(id).Community(); got != commA {
			t.Errorf("expected %s in community %d, got %d", id, commA, got)
		}
	}
	for _, id := range []string{"b1", "b2", "b3", "b4"} {
		if got := g.Node(id).Community(); got != commB {
			t.Errorf("expected %s in community %d, got %d", id, commB, got)
		}
	}
}

func TestLabelCommunitiesSetsLabelForEveryNode(t *testing.T) {
	g := twoClusterGraph(t)
	DetectCommunities(g, DefaultResolution)
	LabelCommunities(g)

	for _, n := range g.Nodes() {
		if n.CommunityLabel() == "" {
			t.Errorf("expected node %s to have a non-empty community label", n.ID)
		}
	}
}

// bridgedClustersGraph builds two densely-connected 4-node clusters joined
// by one weak bridge edge (a1-b1), with b1 given two extra same-cluster
// edges so label propagation's plurality vote keeps it anchored to its own
// cluster rather than crossing the bridge on tie-break alone. The result
// (verified empirically, not derived): label propagation alone converges
// the two clusters into one community at DefaultResolution and below, but
// keeps them apart at a higher resolution, once the resolution-derived
// mergeSmallCommunities threshold no longer folds a still-small cluster
// into its bridge-connected neighbor. That crossover is exactly the
// resolution knob's job, which makes this fixture sensitive to it.
func bridgedClustersGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()
	clusterA := []string{"a1", "a2", "a3", "a4"}
	clusterB := []string{"b1", "b2", "b3", "b4"}
	for _, id := range append(append([]string{}, clusterA...), clusterB...) {
		g.AddNode(&graph.Node{ID: id, Type: graph.NodeTypeFunction, File: "pkg/" + id[:1] + "/x.go"})
	}
	connectAll := func(ids []string) {
		for i := range ids {
			for j := i + 1; j < len(ids); j++ {
				eid := ids[i] + "-" + ids[j]
				must(t, g.AddEdge(&graph.Edge{ID: eid, Type: graph.EdgeTypeCalls, SrcID: ids[i], DstID: ids[j]}))
			}
		}
	}
	connectAll(clusterA)
	connectAll(clusterB)
	must(t, g.AddEdge(&graph.Edge{ID: "bridge", Type: graph.EdgeTypeCalls, SrcID: "a1", DstID: "b1"}))
	must(t, g.AddEdge(&graph.Edge{ID: "boost1", Type: graph.EdgeTypeCalls, SrcID: "b1", DstID: "b2"}))
	must(t, g.AddEdge(&graph.Edge{ID: "boost2", Type: graph.EdgeTypeCalls, SrcID: "b1", DstID: "b3"}))
	return g
}

// TestAnalyzeGraphHonorsResolutionParameter guards against the bug fixed in
// graphify-evolution-followups-2: AnalyzeGraph used to hardcode
// DefaultResolution internally, so `kgraph analyze --resolution X` (without
// --recluster) silently had no effect. This asserts the parameter actually
// reaches DetectCommunities and changes its output.
func TestAnalyzeGraphHonorsResolutionParameter(t *testing.T) {
	gHigh := bridgedClustersGraph(t)
	if err := AnalyzeGraph(gHigh, 2, 2.0); err != nil {
		t.Fatalf("AnalyzeGraph(resolution=2.0) error = %v", err)
	}
	highCount := len(gHigh.Communities())

	gLow := bridgedClustersGraph(t)
	if err := AnalyzeGraph(gLow, 2, DefaultResolution); err != nil {
		t.Fatalf("AnalyzeGraph(resolution=default) error = %v", err)
	}
	lowCount := len(gLow.Communities())

	if highCount <= lowCount {
		t.Fatalf("expected a higher resolution to yield more communities than the default, got high=%d default=%d", highCount, lowCount)
	}
}

func TestAnalyzeGraphOrchestratesFullPipeline(t *testing.T) {
	g := twoClusterGraph(t)
	if err := AnalyzeGraph(g, 2, DefaultResolution); err != nil {
		t.Fatalf("AnalyzeGraph() error = %v", err)
	}
	for _, n := range g.Nodes() {
		if n.Degree() == 0 {
			t.Errorf("expected node %s to have a nonzero degree after AnalyzeGraph", n.ID)
		}
		if n.Community() == -1 {
			t.Errorf("expected node %s to have a community assigned after AnalyzeGraph", n.ID)
		}
		if n.CommunityLabel() == "" {
			t.Errorf("expected node %s to have a community label after AnalyzeGraph", n.ID)
		}
	}
	godCount := 0
	for _, n := range g.Nodes() {
		if n.IsGodNode() {
			godCount++
		}
	}
	if godCount != 2 {
		t.Errorf("expected 2 god nodes (godNodeCount=2), got %d", godCount)
	}
}
