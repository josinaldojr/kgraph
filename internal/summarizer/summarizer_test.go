package summarizer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// buildFixtureGraph creates a small on-disk Go-like source file and a
// matching graph: one Package node and two Function nodes in the same
// file, so file/module-level aggregation has something to aggregate.
func buildFixtureGraph(t *testing.T) (*graph.Graph, string) {
	t.Helper()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "demo.go")
	src := "package demo\n\nfunc Foo() {\n\treturn\n}\n\nfunc Bar() {\n\treturn\n}\n"
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatalf("writing fixture source: %v", err)
	}

	g := graph.New()
	g.AddNode(&graph.Node{
		ID: "pkg:demo", Type: graph.NodeTypePackage,
		Hash: "pkg-hash", Properties: map[string]any{"name": "demo"},
	})
	g.AddNode(&graph.Node{
		ID: "demo.Foo()", Type: graph.NodeTypeFunction,
		File: srcPath, LineStart: 3, LineEnd: 5,
		Signature: "func Foo()", Hash: "foo-hash-1",
		Properties: map[string]any{"name": "Foo", "package": "demo"},
	})
	g.AddNode(&graph.Node{
		ID: "demo.Bar()", Type: graph.NodeTypeFunction,
		File: srcPath, LineStart: 7, LineEnd: 9,
		Signature: "func Bar()", Hash: "bar-hash-1",
		Properties: map[string]any{"name": "Bar", "package": "demo"},
	})
	return g, srcPath
}

func TestPendingNodesIncludesSourceText(t *testing.T) {
	g, _ := buildFixtureGraph(t)
	s := openTestStore(t)

	pending, err := PendingSummaries(g, s, LevelNode, 0)
	if err != nil {
		t.Fatalf("PendingSummaries() error = %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending nodes, got %d", len(pending))
	}
	for _, p := range pending {
		if p.Source == "" {
			t.Errorf("expected non-empty source for %s", p.ID)
		}
	}
}

func TestApplySummariesHashGating(t *testing.T) {
	g, _ := buildFixtureGraph(t)
	s := openTestStore(t)

	res, err := ApplySummaries(g, s, LevelNode, "test-model", []AppliedSummary{
		{ID: "demo.Foo()", Hash: "foo-hash-1", Summary: "Does foo things."},
		{ID: "demo.Bar()", Hash: "WRONG-HASH", Summary: "Does bar things."},
	})
	if err != nil {
		t.Fatalf("ApplySummaries() error = %v", err)
	}
	if len(res.Applied) != 1 || res.Applied[0] != "demo.Foo()" {
		t.Errorf("expected only demo.Foo() applied, got %+v", res.Applied)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].ID != "demo.Bar()" {
		t.Errorf("expected demo.Bar() skipped for hash mismatch, got %+v", res.Skipped)
	}

	pending, err := PendingSummaries(g, s, LevelNode, 0)
	if err != nil {
		t.Fatalf("PendingSummaries() error = %v", err)
	}
	if len(pending) != 1 || pending[0].ID != "demo.Bar()" {
		t.Fatalf("expected only demo.Bar() still pending, got %+v", pending)
	}
}

func TestFileAndModuleLevelAggregation(t *testing.T) {
	g, srcPath := buildFixtureGraph(t)
	s := openTestStore(t)

	// Files aren't pending until every child node is summarized.
	filesPending, err := PendingSummaries(g, s, LevelFile, 0)
	if err != nil {
		t.Fatalf("PendingSummaries(file) error = %v", err)
	}
	if len(filesPending) != 0 {
		t.Fatalf("expected no pending files before children are summarized, got %+v", filesPending)
	}

	if _, err := ApplySummaries(g, s, LevelNode, "test-model", []AppliedSummary{
		{ID: "demo.Foo()", Hash: "foo-hash-1", Summary: "Does foo things."},
		{ID: "demo.Bar()", Hash: "bar-hash-1", Summary: "Does bar things."},
	}); err != nil {
		t.Fatalf("ApplySummaries(node) error = %v", err)
	}

	filesPending, err = PendingSummaries(g, s, LevelFile, 0)
	if err != nil {
		t.Fatalf("PendingSummaries(file) error = %v", err)
	}
	if len(filesPending) != 1 || filesPending[0].ID != srcPath {
		t.Fatalf("expected file %s pending, got %+v", srcPath, filesPending)
	}

	fileApply, err := ApplySummaries(g, s, LevelFile, "test-model", []AppliedSummary{
		{ID: srcPath, Hash: filesPending[0].Hash, Summary: "demo.go declares Foo and Bar."},
	})
	if err != nil {
		t.Fatalf("ApplySummaries(file) error = %v", err)
	}
	if len(fileApply.Applied) != 1 {
		t.Fatalf("expected file summary applied, got %+v", fileApply)
	}

	modulesPending, err := PendingSummaries(g, s, LevelModule, 0)
	if err != nil {
		t.Fatalf("PendingSummaries(module) error = %v", err)
	}
	if len(modulesPending) != 1 || modulesPending[0].ID != graph.PackageID("demo") {
		t.Fatalf("expected module pkg:demo pending, got %+v", modulesPending)
	}
}

func TestSearchNodesRanksByTermOverlap(t *testing.T) {
	g, _ := buildFixtureGraph(t)
	s := openTestStore(t)

	if _, err := ApplySummaries(g, s, LevelNode, "test-model", []AppliedSummary{
		{ID: "demo.Foo()", Hash: "foo-hash-1", Summary: "Formats an invoice as PDF."},
		{ID: "demo.Bar()", Hash: "bar-hash-1", Summary: "Sends a notification email."},
	}); err != nil {
		t.Fatalf("ApplySummaries() error = %v", err)
	}

	results, err := SearchNodes(g, s, "invoice pdf", 5)
	if err != nil {
		t.Fatalf("SearchNodes() error = %v", err)
	}
	if len(results) == 0 || results[0].Node.ID != "demo.Foo()" {
		t.Fatalf("expected demo.Foo() to rank first for 'invoice pdf', got %+v", results)
	}
}

func TestSearchNodesFallsBackToSignature(t *testing.T) {
	g, _ := buildFixtureGraph(t)
	s := openTestStore(t)
	// No summaries applied at all — search must still match on signature/name.

	results, err := SearchNodes(g, s, "Foo", 5)
	if err != nil {
		t.Fatalf("SearchNodes() error = %v", err)
	}
	if len(results) == 0 || results[0].Node.ID != "demo.Foo()" {
		t.Fatalf("expected demo.Foo() to match on name/signature fallback, got %+v", results)
	}
}
