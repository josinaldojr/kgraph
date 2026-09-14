package export

import (
	"strings"
	"testing"

	"kgraph/internal/analytics"
	"kgraph/internal/enrich"
	"kgraph/internal/graph"
)

// twoCommunityGraph builds two densely-connected clusters plus a single
// "bridge" edge between them, so god-node/community/surprising-connection
// detection all have something real to report on.
func twoCommunityGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()
	clusterA := []string{"a1", "a2", "a3", "a4"}
	clusterB := []string{"b1", "b2", "b3", "b4"}
	for _, id := range append(append([]string{}, clusterA...), clusterB...) {
		g.AddNode(&graph.Node{ID: id, Type: graph.NodeTypeFunction, File: "pkg/" + id[:1] + "/x.go"})
	}
	connect := func(ids []string) {
		for i := range ids {
			for j := i + 1; j < len(ids); j++ {
				must(t, g.AddEdge(&graph.Edge{ID: ids[i] + "-" + ids[j], Type: graph.EdgeTypeCalls, SrcID: ids[i], DstID: ids[j]}))
			}
		}
	}
	connect(clusterA)
	connect(clusterB)
	must(t, g.AddEdge(&graph.Edge{ID: "bridge", Type: graph.EdgeTypeCalls, SrcID: "a1", DstID: "b1"}))

	enrich.AnnotateConfidence(g)
	if err := analytics.AnalyzeGraph(g, 2, analytics.DefaultResolution); err != nil {
		t.Fatalf("AnalyzeGraph() error = %v", err)
	}
	return g
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateReportIncludesAllSections(t *testing.T) {
	g := twoCommunityGraph(t)

	out, err := GenerateReport(g, nil)
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}
	for _, want := range []string{"## God Nodes", "## Communities", "## Surprising Connections", "## Suggested Questions"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected report to contain section %q, got:\n%s", want, out)
		}
	}
}

func TestGenerateReportListsBridgeEdgeAsSurprising(t *testing.T) {
	g := twoCommunityGraph(t)

	out, err := GenerateReport(g, nil)
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}
	if !strings.Contains(out, "`a1`") || !strings.Contains(out, "`b1`") {
		t.Fatalf("expected the a1->b1 bridge edge to be listed as a surprising connection, got:\n%s", out)
	}
}

func TestGenerateReportSuggestsQuestions(t *testing.T) {
	g := twoCommunityGraph(t)

	out, err := GenerateReport(g, nil)
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}
	idx := strings.Index(out, "## Suggested Questions")
	if idx == -1 {
		t.Fatal("missing Suggested Questions section")
	}
	section := out[idx:]
	if strings.Count(section, "\n- ") == 0 {
		t.Errorf("expected at least one suggested question, got:\n%s", section)
	}
}
