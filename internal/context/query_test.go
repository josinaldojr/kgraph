package context

import (
	"strings"
	"testing"

	"kgraph/internal/summarizer"
)

func TestQueryFindsRelevantNodeAndExpands(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	if _, err := summarizer.ApplySummaries(g, s, summarizer.LevelNode, "test", []summarizer.AppliedSummary{
		{ID: "demo.A()", Hash: "a1", Summary: "Handles invoice creation for a customer order."},
	}); err != nil {
		t.Fatalf("ApplySummaries() error = %v", err)
	}

	out, err := Query(g, s, "invoice creation", 1, 0)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if !strings.Contains(out, "Query: invoice creation") {
		t.Fatalf("expected question echoed in output, got:\n%s", out)
	}
	if !strings.Contains(out, "demo.A()") {
		t.Fatalf("expected the matching node in output, got:\n%s", out)
	}
}

func TestQueryNoMatchReturnsError(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	if _, err := Query(g, s, "zzz_no_such_term_zzz", 1, 0); err == nil {
		t.Fatal("expected an error when no node matches the query")
	}
}

func TestQueryCommunityAwareRelevanceScoping(t *testing.T) {
	g := chainGraph()
	s := openTestStore(t)

	if _, err := summarizer.ApplySummaries(g, s, summarizer.LevelNode, "test", []summarizer.AppliedSummary{
		{ID: "demo.A()", Hash: "a1", Summary: "gizmo widget entrypoint"},
	}); err != nil {
		t.Fatalf("ApplySummaries() error = %v", err)
	}
	g.Node("demo.A()").Properties["community"] = 1
	g.Node("demo.B()").Properties["community"] = 1
	g.Node("demo.Order").Properties["community"] = 2

	out, err := Query(g, s, "gizmo widget", 1, 0)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if !strings.Contains(out, "demo.B()") {
		t.Fatalf("expected same-community neighbor demo.B() to appear in query output, got:\n%s", out)
	}
}
