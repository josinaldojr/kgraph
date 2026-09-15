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
// edges. The result (verified empirically, not derived): Louvain keeps the
// two clusters apart at and above DefaultResolution — the bridge is too
// weak relative to each clique's internal density for modularity to prefer
// merging them — but at a low enough resolution (below DefaultResolution)
// the per-community penalty term shrinks enough that merging the two
// clusters into one community becomes the modularity-maximizing move. That
// crossover is exactly the resolution knob's job, which makes this fixture
// sensitive to it.
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
// reaches DetectCommunities and changes its output: a low-enough resolution
// merges bridgedClustersGraph's two cliques into one community, while
// DefaultResolution keeps them apart.
func TestAnalyzeGraphHonorsResolutionParameter(t *testing.T) {
	gLow := bridgedClustersGraph(t)
	if err := AnalyzeGraph(gLow, 2, 0.1); err != nil {
		t.Fatalf("AnalyzeGraph(resolution=0.1) error = %v", err)
	}
	lowCount := len(gLow.Communities())

	gDefault := bridgedClustersGraph(t)
	if err := AnalyzeGraph(gDefault, 2, DefaultResolution); err != nil {
		t.Fatalf("AnalyzeGraph(resolution=default) error = %v", err)
	}
	defaultCount := len(gDefault.Communities())

	if lowCount >= defaultCount {
		t.Fatalf("expected a lower resolution to yield fewer communities than the default, got low=%d default=%d", lowCount, defaultCount)
	}
}

// TestDetectCommunitiesDeterministic guards graph-analytics' "Deterministic
// output for a fixed input" scenario: running detection twice on the same
// graph and resolution, without any edit in between, must produce identical
// per-node community assignments — node evaluation order and modularity-gain
// tie-breaking must not depend on map iteration order or any other
// non-deterministic source.
func TestDetectCommunitiesDeterministic(t *testing.T) {
	for _, resolution := range []float64{0.1, DefaultResolution, 2.0, 3.0} {
		g1 := bridgedClustersGraph(t)
		DetectCommunities(g1, resolution)
		g2 := bridgedClustersGraph(t)
		DetectCommunities(g2, resolution)

		for _, id := range []string{"a1", "a2", "a3", "a4", "b1", "b2", "b3", "b4"} {
			c1, c2 := g1.Node(id).Community(), g2.Node(id).Community()
			if c1 != c2 {
				t.Errorf("resolution=%v: node %s got community %d on run 1 but %d on run 2", resolution, id, c1, c2)
			}
		}
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
