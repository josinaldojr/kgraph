package context

import (
	"strings"
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
)

func TestExplainIncludesRationaleCommunityAndConfidence(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	rationale := &graph.Node{
		ID:         graph.RationaleID("f.go", "1", "WHY"),
		Type:       graph.NodeTypeRationale,
		Properties: map[string]any{"kind": "WHY", "text": "Keeps the call chain simple for testability"},
	}
	g.AddNode(rationale)
	if err := g.AddEdge(&graph.Edge{
		ID: graph.EdgeID(graph.EdgeTypeExplains, rationale.ID, "demo.A()"), Type: graph.EdgeTypeExplains,
		SrcID: rationale.ID, DstID: "demo.A()", Confidence: graph.ConfidenceInferred,
	}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}
	for _, e := range g.OutEdges("demo.A()") {
		e.Confidence = graph.ConfidenceExtracted
	}
	g.Node("demo.A()").Properties["community"] = 4
	g.Node("demo.A()").Properties["community_label"] = "Demo"
	g.Node("demo.A()").Properties["god_node"] = true
	g.Node("demo.A()").Properties["degree"] = 7

	out, err := Explain(g, s, "A", 1)
	if err != nil {
		t.Fatalf("Explain() error = %v", err)
	}
	if !strings.HasPrefix(out, "# demo.A() ") {
		t.Fatalf("expected explain output to start with the resolved node, got:\n%s", out)
	}
	for _, want := range []string{
		"Keeps the call chain simple for testability",
		"Community: Demo (id 4)",
		"God node: yes",
		"(EXTRACTED)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected explain output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestExplainResolvesByName(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	out, err := Explain(g, s, "B", 1)
	if err != nil {
		t.Fatalf("Explain() error = %v", err)
	}
	if !strings.HasPrefix(out, "# demo.B() ") {
		t.Fatalf("expected explain to resolve by name, got:\n%s", out)
	}
}

func TestExplainWithHopsIncludesFartherNodes(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	out1, err := Explain(g, s, "A", 1)
	if err != nil {
		t.Fatalf("Explain(hops=1) error = %v", err)
	}
	if strings.Contains(out1, "demo.C()") {
		t.Fatalf("expected hops=1 not to reach demo.C() (2 hops away), got:\n%s", out1)
	}

	out2, err := Explain(g, s, "A", 2)
	if err != nil {
		t.Fatalf("Explain(hops=2) error = %v", err)
	}
	if !strings.Contains(out2, "demo.C()") {
		t.Fatalf("expected hops=2 to reach demo.C(), got:\n%s", out2)
	}
}

func TestPromptRendersMarkdownSections(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	rationale := &graph.Node{
		ID:         graph.RationaleID("f.go", "1", "NOTE"),
		Type:       graph.NodeTypeRationale,
		Properties: map[string]any{"kind": "NOTE", "text": "Kept trivial on purpose"},
	}
	g.AddNode(rationale)
	if err := g.AddEdge(&graph.Edge{
		ID: graph.EdgeID(graph.EdgeTypeExplains, rationale.ID, "demo.A()"), Type: graph.EdgeTypeExplains,
		SrcID: rationale.ID, DstID: "demo.A()", Confidence: graph.ConfidenceInferred,
	}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}
	g.Node("demo.A()").Properties["community"] = 1
	g.Node("demo.A()").Properties["community_label"] = "Demo"

	out, err := Prompt(g, s, "A", 0)
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	for _, want := range []string{
		"## Context: demo.A()", "### Direct Relations", "### Related Nodes",
		"### Rationale", "### Community", "Kept trivial on purpose",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected prompt output to contain %q, got:\n%s", want, out)
		}
	}
	// demo.C() is 2 hops from A (A->B->C) — one level past Direct Relations,
	// so it belongs in Related Nodes, not Direct Relations.
	if !strings.Contains(out, "demo.C()") {
		t.Errorf("expected Related Nodes to include demo.C() (2 hops out), got:\n%s", out)
	}
}

func TestPromptRespectsTokenBudget(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	full, err := Prompt(g, s, "A", 0)
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if !strings.Contains(full, "### Related Nodes") {
		t.Fatalf("expected the unbudgeted prompt to include Related Nodes, got:\n%s", full)
	}

	tight, err := Prompt(g, s, "A", 10)
	if err != nil {
		t.Fatalf("Prompt(maxTokens=10) error = %v", err)
	}
	if len(tight) >= len(full) {
		t.Errorf("expected a tight token budget to produce shorter output than the default, got %d vs %d bytes", len(tight), len(full))
	}
	if !strings.Contains(tight, "## Context: demo.A()") {
		t.Errorf("expected the target's own Context section to survive a tight budget, got:\n%s", tight)
	}
}
