package context

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/store"
	"github.com/josinaldojr/kgraph/internal/summarizer"
)

// chainGraph builds A -> calls -> B -> calls -> C, plus a struct/table
// pair off of A, so tests exercise both the calls chain (multi-hop) and
// the target's grouped relation rendering.
func chainGraph() *graph.Graph {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "pkg:demo", Type: graph.NodeTypePackage, Hash: "pkg"})
	g.AddNode(&graph.Node{ID: "demo.A()", Type: graph.NodeTypeFunction, Signature: "func A()", Hash: "a1", Properties: map[string]any{"name": "A"}})
	g.AddNode(&graph.Node{ID: "demo.B()", Type: graph.NodeTypeFunction, Signature: "func B()", Hash: "b1", Properties: map[string]any{"name": "B"}})
	g.AddNode(&graph.Node{ID: "demo.C()", Type: graph.NodeTypeFunction, Signature: "func C()", Hash: "c1", Properties: map[string]any{"name": "C"}})
	g.AddNode(&graph.Node{ID: "demo.Order", Type: graph.NodeTypeStruct, Signature: "type Order struct", Hash: "o1", Properties: map[string]any{"name": "Order"}})
	g.AddNode(&graph.Node{ID: "table:orders", Type: graph.NodeTypeTable, Hash: "t1"})

	mustAdd := func(e *graph.Edge) {
		if err := g.AddEdge(e); err != nil {
			panic(err)
		}
	}
	mustAdd(&graph.Edge{ID: "e1", Type: graph.EdgeTypeCalls, SrcID: "demo.A()", DstID: "demo.B()"})
	mustAdd(&graph.Edge{ID: "e2", Type: graph.EdgeTypeCalls, SrcID: "demo.B()", DstID: "demo.C()"})
	mustAdd(&graph.Edge{ID: "e3", Type: graph.EdgeTypeMapsToTable, SrcID: "demo.Order", DstID: "table:orders"})
	return g
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestGetContextExactMatchResolution(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	out, err := GetContext(g, s, "A", 1, 0)
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}
	if !strings.HasPrefix(out, "# demo.A() ") {
		t.Fatalf("expected output to start with target demo.A(), got:\n%s", out)
	}
}

func TestGetContextFallsBackToLexicalSearch(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	if _, err := summarizer.ApplySummaries(g, s, summarizer.LevelNode, "test", []summarizer.AppliedSummary{
		{ID: "demo.C()", Hash: "c1", Summary: "Formats an invoice as PDF."},
	}); err != nil {
		t.Fatalf("ApplySummaries() error = %v", err)
	}

	// "invoice" matches no file/struct/function name directly, only C()'s
	// summary — this must go through the SearchNodes fallback.
	out, err := GetContext(g, s, "invoice", 1, 0)
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}
	if !strings.HasPrefix(out, "# demo.C() ") {
		t.Fatalf("expected fallback search to resolve demo.C(), got:\n%s", out)
	}
}

func TestGetContextDefaultsApplied(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	// hops=0 -> DefaultHops(2) should reach C (2 hops from A); hops=1
	// would stop at B.
	out, err := GetContext(g, s, "A", 0, 0)
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}
	if !strings.Contains(out, "demo.C()") {
		t.Fatalf("expected default hops=2 to reach demo.C() from demo.A(), got:\n%s", out)
	}
}

func TestGetContextTruncatesUnderTightBudget(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	// A tiny budget still returns the target's own section, but must
	// truncate (and report truncating) the related-nodes list.
	out, err := GetContext(g, s, "A", 2, 20)
	if err != nil {
		t.Fatalf("GetContext() error = %v", err)
	}
	if !strings.HasPrefix(out, "# demo.A() ") {
		t.Fatalf("expected target section always present even under a tight budget, got:\n%s", out)
	}
	if !strings.Contains(out, "omitted") {
		t.Fatalf("expected output to report omitted related nodes under a tight budget, got:\n%s", out)
	}
}
