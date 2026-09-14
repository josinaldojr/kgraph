package enrich

import (
	"strings"
	"testing"

	"kgraph/internal/graph"
)

func TestExtractRationaleGoNote(t *testing.T) {
	content := strings.Join([]string{
		"package auth",
		"",
		"// NOTE: Uses stateless JWT for horizontal scaling",
		"func Login() {}",
	}, "\n")

	nodes := ExtractRationale(graph.New(), "auth.go", content)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 rationale node, got %d", len(nodes))
	}
	n := nodes[0]
	if n.Type != graph.NodeTypeRationale {
		t.Errorf("expected NodeTypeRationale, got %s", n.Type)
	}
	if kind, _ := n.Properties["kind"].(string); kind != "NOTE" {
		t.Errorf("expected kind=NOTE, got %q", kind)
	}
	if text, _ := n.Properties["text"].(string); text != "Uses stateless JWT for horizontal scaling" {
		t.Errorf("unexpected text: %q", text)
	}
	if n.LineStart != 3 {
		t.Errorf("expected LineStart=3, got %d", n.LineStart)
	}
}

func TestExtractRationalePythonWhy(t *testing.T) {
	content := strings.Join([]string{
		"def get_user(id):",
		"    # WHY: This avoids N+1 queries on the hot path",
		"    return db.get(id)",
	}, "\n")

	nodes := ExtractRationale(graph.New(), "models.py", content)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 rationale node, got %d", len(nodes))
	}
	if kind, _ := nodes[0].Properties["kind"].(string); kind != "WHY" {
		t.Errorf("expected kind=WHY, got %q", kind)
	}
	if text, _ := nodes[0].Properties["text"].(string); text != "This avoids N+1 queries on the hot path" {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestExtractRationaleTypeScriptHack(t *testing.T) {
	content := strings.Join([]string{
		"export function fetchData() {",
		"  // HACK: Working around upstream API bug, remove after v3",
		"  return fetch('/api');",
		"}",
	}, "\n")

	nodes := ExtractRationale(graph.New(), "api.ts", content)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 rationale node, got %d", len(nodes))
	}
	if kind, _ := nodes[0].Properties["kind"].(string); kind != "HACK" {
		t.Errorf("expected kind=HACK, got %q", kind)
	}
}

func TestExtractRationaleMultiLineContinuation(t *testing.T) {
	content := strings.Join([]string{
		"package auth",
		"",
		"// NOTE: Uses stateless JWT for horizontal scaling,",
		"// avoiding sticky sessions across replicas.",
		"func Login() {}",
	}, "\n")

	nodes := ExtractRationale(graph.New(), "auth.go", content)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 rationale node, got %d", len(nodes))
	}
	n := nodes[0]
	want := "Uses stateless JWT for horizontal scaling, avoiding sticky sessions across replicas."
	if text, _ := n.Properties["text"].(string); text != want {
		t.Errorf("unexpected joined text: %q, want %q", text, want)
	}
	if n.LineStart != 3 {
		t.Errorf("expected LineStart=3, got %d", n.LineStart)
	}
	if n.LineEnd != 4 {
		t.Errorf("expected LineEnd=4 (last continuation line), got %d", n.LineEnd)
	}
}

func TestExtractRationaleAdjacentTagsStaySeparate(t *testing.T) {
	content := strings.Join([]string{
		"// NOTE: first rationale",
		"// WHY: second, unrelated rationale",
		"func F() {}",
	}, "\n")

	nodes := ExtractRationale(graph.New(), "f.go", content)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 separate rationale nodes, got %d: %+v", len(nodes), nodes)
	}
	if kind, _ := nodes[0].Properties["kind"].(string); kind != "NOTE" {
		t.Errorf("expected first node kind=NOTE, got %q", kind)
	}
	if text, _ := nodes[0].Properties["text"].(string); text != "first rationale" {
		t.Errorf("expected first node text unmerged, got %q", text)
	}
	if kind, _ := nodes[1].Properties["kind"].(string); kind != "WHY" {
		t.Errorf("expected second node kind=WHY, got %q", kind)
	}
	if text, _ := nodes[1].Properties["text"].(string); text != "second, unrelated rationale" {
		t.Errorf("expected second node text unmerged, got %q", text)
	}
}

func TestExtractRationaleContinuationStopsAtBlankLine(t *testing.T) {
	content := strings.Join([]string{
		"// NOTE: only this line",
		"//",
		"// not part of the rationale",
		"func F() {}",
	}, "\n")

	nodes := ExtractRationale(graph.New(), "f.go", content)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 rationale node, got %d", len(nodes))
	}
	if text, _ := nodes[0].Properties["text"].(string); text != "only this line" {
		t.Errorf("expected continuation to stop at the blank comment line, got %q", text)
	}
}

func TestRationaleIDIsDeterministic(t *testing.T) {
	content := "// NOTE: stable id\nfunc F() {}\n"
	first := ExtractRationale(graph.New(), "f.go", content)
	second := ExtractRationale(graph.New(), "f.go", content)
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected 1 node each run, got %d and %d", len(first), len(second))
	}
	if first[0].ID != second[0].ID {
		t.Errorf("expected identical IDs across runs, got %q vs %q", first[0].ID, second[0].ID)
	}
}

func TestExtractDocstringsGo(t *testing.T) {
	content := strings.Join([]string{
		"package auth",
		"",
		"// Login authenticates a user and returns a session token.",
		"// It rejects locked accounts outright.",
		"func Login() {}",
	}, "\n")

	nodes := ExtractDocstrings(graph.New(), "auth.go", content)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 docstring node, got %d", len(nodes))
	}
	if kind, _ := nodes[0].Properties["kind"].(string); kind != "DOCSTRING" {
		t.Errorf("expected kind=DOCSTRING, got %q", kind)
	}
	if !strings.Contains(nodes[0].Properties["text"].(string), "authenticates a user") {
		t.Errorf("unexpected docstring text: %q", nodes[0].Properties["text"])
	}
}

func TestExtractDocstringsGoSkipsRationaleTaggedComment(t *testing.T) {
	content := "// NOTE: single tagged line\nfunc F() {}\n"
	nodes := ExtractDocstrings(graph.New(), "f.go", content)
	if len(nodes) != 0 {
		t.Fatalf("expected rationale-tagged single-line comment not to double as a docstring, got %d nodes", len(nodes))
	}
}

func TestExtractDocstringsPython(t *testing.T) {
	content := strings.Join([]string{
		"def get_user(id):",
		`    """Fetch a user by ID from the primary datastore."""`,
		"    return db.get(id)",
	}, "\n")

	nodes := ExtractDocstrings(graph.New(), "models.py", content)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 docstring node, got %d", len(nodes))
	}
	if text, _ := nodes[0].Properties["text"].(string); text != "Fetch a user by ID from the primary datastore." {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestExtractDocstringsTypeScriptJSDoc(t *testing.T) {
	content := strings.Join([]string{
		"/**",
		" * Fetches data from the remote API.",
		" */",
		"export function fetchData() {}",
	}, "\n")

	nodes := ExtractDocstrings(graph.New(), "api.ts", content)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 docstring node, got %d", len(nodes))
	}
	if text, _ := nodes[0].Properties["text"].(string); text != "Fetches data from the remote API." {
		t.Errorf("unexpected text: %q", text)
	}
}

func TestLinkRationaleCreatesExplainsEdge(t *testing.T) {
	g := graph.New()
	target := &graph.Node{ID: "pkg.Foo()", Type: graph.NodeTypeFunction, File: "f.go", LineStart: 2}
	g.AddNode(target)
	r := rationaleNode("f.go", 1, 1, "NOTE", "some rationale")
	g.AddNode(r)

	LinkRationale(g, []*graph.Node{r}, target)

	edges := g.OutEdges(r.ID)
	if len(edges) != 1 {
		t.Fatalf("expected 1 explains edge, got %d", len(edges))
	}
	if edges[0].Type != graph.EdgeTypeExplains {
		t.Errorf("expected explains edge, got %s", edges[0].Type)
	}
	if edges[0].DstID != target.ID {
		t.Errorf("expected edge to target %s, got %s", target.ID, edges[0].DstID)
	}
}
