package context

import (
	"strings"
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
)

func TestGetContextIncludesRationaleSection(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	rationale := &graph.Node{
		ID:   graph.RationaleID("f.go", "1", "NOTE"),
		Type: graph.NodeTypeRationale,
		Properties: map[string]any{
			"kind": "NOTE",
			"text": "Uses stateless JWT for horizontal scaling",
		},
	}
	g.AddNode(rationale)
	if err := g.AddEdge(&graph.Edge{
		ID: graph.EdgeID(graph.EdgeTypeExplains, rationale.ID, "demo.A()"), Type: graph.EdgeTypeExplains,
		SrcID: rationale.ID, DstID: "demo.A()", Confidence: graph.ConfidenceInferred,
	}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}

	out, err := GetContext(g, s, "A", 1, 0)
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}
	if !strings.Contains(out, "Rationale:") || !strings.Contains(out, "Uses stateless JWT for horizontal scaling") {
		t.Fatalf("expected rationale section in output, got:\n%s", out)
	}
}

func TestGetContextIncludesConfidenceTag(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)
	// Manually set confidence on the calls edge — a real build would have
	// run AnnotateConfidence already.
	for _, e := range g.OutEdges("demo.A()") {
		e.Confidence = graph.ConfidenceExtracted
	}

	out, err := GetContext(g, s, "A", 1, 0)
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}
	if !strings.Contains(out, "(EXTRACTED)") {
		t.Fatalf("expected related-node line to include confidence tag, got:\n%s", out)
	}
}

func TestGetContextIncludesCommunityAndGodNode(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	target := g.Node("demo.A()")
	target.Properties["community"] = 2
	target.Properties["community_label"] = "Demo"
	target.Properties["god_node"] = true
	target.Properties["degree"] = 5

	out, err := GetContext(g, s, "A", 1, 0)
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}
	if !strings.Contains(out, "Community: Demo (id 2)") {
		t.Fatalf("expected community label in output, got:\n%s", out)
	}
	if !strings.Contains(out, "God node: yes") {
		t.Fatalf("expected god-node flag in output, got:\n%s", out)
	}
}

func TestExpandSubgraphBoostsSameCommunityNeighbors(t *testing.T) {
	g := chainGraph()
	// demo.A()'s community is 1; demo.B() shares it (should get a boost),
	// demo.Order (reached from A at hop 1 via a different, unrelated edge
	// in a real graph) is in a different community. Here we just check
	// that a same-community neighbor outranks an otherwise-equal-weight
	// different-community neighbor.
	g.Node("demo.A()").Properties["community"] = 1
	g.Node("demo.B()").Properties["community"] = 1
	g.Node("demo.Order").Properties["community"] = 2

	target := g.Node("demo.A()")
	found := expandSubgraph(g, target, 1)

	b := found["demo.B()"]
	if b == nil {
		t.Fatal("expected demo.B() to be discovered at hop 1")
	}
	if b.Weight <= edgeWeight(graph.EdgeTypeCalls) {
		t.Errorf("expected same-community neighbor to get a relevance boost above the base edge weight, got %d", b.Weight)
	}
}
