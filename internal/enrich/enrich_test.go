package enrich

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kgraph/internal/graph"
)

// writeFixtureFile writes content to name under a fresh temp dir and
// returns the absolute path — EnrichGraph reads source files off disk via
// os.ReadFile, so its tests need real files, not in-memory content.
func writeFixtureFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}
	return path
}

func TestEnrichGraphLinksRationaleToNearestEntity(t *testing.T) {
	path := writeFixtureFile(t, "demo.go", strings.Join([]string{
		"package demo",
		"",
		"// NOTE: Kept trivial on purpose",
		"func Foo() {}",
		"",
		"func Bar() {}",
	}, "\n"))

	g := graph.New()
	foo := &graph.Node{ID: "demo.Foo()", Type: graph.NodeTypeFunction, File: path, LineStart: 4, LineEnd: 4, Signature: "func Foo()"}
	bar := &graph.Node{ID: "demo.Bar()", Type: graph.NodeTypeFunction, File: path, LineStart: 6, LineEnd: 6, Signature: "func Bar()"}
	g.AddNode(foo)
	g.AddNode(bar)
	if err := g.AddEdge(&graph.Edge{ID: "e1", Type: graph.EdgeTypeCalls, SrcID: foo.ID, DstID: bar.ID}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}

	warnings, err := EnrichGraph(g)
	if err != nil {
		t.Fatalf("EnrichGraph() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	var rationale *graph.Node
	for _, n := range g.Nodes() {
		if n.Type == graph.NodeTypeRationale {
			rationale = n
		}
	}
	if rationale == nil {
		t.Fatal("expected EnrichGraph to add a Rationale node")
	}
	if kind, _ := rationale.Properties["kind"].(string); kind != "NOTE" {
		t.Errorf("expected kind=NOTE, got %q", kind)
	}

	edges := g.OutEdges(rationale.ID)
	if len(edges) != 1 || edges[0].Type != graph.EdgeTypeExplains || edges[0].DstID != foo.ID {
		t.Fatalf("expected an explains edge from the rationale to demo.Foo() (the nearest entity at/after the comment), got %+v", edges)
	}

	if edges[0].Confidence != graph.ConfidenceInferred {
		t.Errorf("expected the explains edge to carry INFERRED confidence, got %q", edges[0].Confidence)
	}
	callEdge := g.OutEdges(foo.ID)
	if len(callEdge) != 1 || callEdge[0].Confidence != graph.ConfidenceExtracted {
		t.Errorf("expected AnnotateConfidence to have run over every edge, got %+v", callEdge)
	}
}

func TestEnrichGraphReportsMissingFileAsWarningNotError(t *testing.T) {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "demo.Foo()", Type: graph.NodeTypeFunction, File: filepath.Join(t.TempDir(), "does-not-exist.go"), LineStart: 1, LineEnd: 1})

	warnings, err := EnrichGraph(g)
	if err != nil {
		t.Fatalf("expected a missing source file to be non-fatal, got error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected exactly 1 warning for the missing file, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "does-not-exist.go") {
		t.Errorf("expected the warning to name the missing file, got %q", warnings[0])
	}
}

func TestNearestEntityPicksEnclosingWhenCommentIsAfterLastEntity(t *testing.T) {
	entities := []*graph.Node{
		{ID: "a", LineStart: 1},
		{ID: "b", LineStart: 10},
	}
	got := nearestEntity(entities, 20)
	if got == nil || got.ID != "b" {
		t.Errorf("expected the enclosing (last) entity when the comment is after every entity, got %+v", got)
	}
}

func TestNearestEntityPicksNextWhenCommentPrecedesAnEntity(t *testing.T) {
	entities := []*graph.Node{
		{ID: "a", LineStart: 1},
		{ID: "b", LineStart: 10},
	}
	got := nearestEntity(entities, 5)
	if got == nil || got.ID != "b" {
		t.Errorf("expected the next entity at/after the comment line, got %+v", got)
	}
}

func TestNearestEntityReturnsNilForEmptyEntities(t *testing.T) {
	if got := nearestEntity(nil, 1); got != nil {
		t.Errorf("expected nil for an empty entities slice, got %+v", got)
	}
}
