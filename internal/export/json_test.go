package export

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/josinaldojr/kgraph/internal/analytics"
	"github.com/josinaldojr/kgraph/internal/enrich"
	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/store"
	"github.com/josinaldojr/kgraph/internal/summarizer"
)

func testGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()
	g.AddNode(&graph.Node{ID: "a", Type: graph.NodeTypePackage, Hash: "h1", Properties: map[string]any{"name": "a"}})
	g.AddNode(&graph.Node{ID: "b", Type: graph.NodeTypePackage, Hash: "h2", Properties: map[string]any{"name": "b"}})
	if err := g.AddEdge(&graph.Edge{ID: "e1", Type: graph.EdgeTypeImports, SrcID: "a", DstID: "b"}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}
	enrich.AnnotateConfidence(g)
	if err := analytics.AnalyzeGraph(g, 1, analytics.DefaultResolution); err != nil {
		t.Fatalf("AnalyzeGraph() error = %v", err)
	}
	return g
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestToJSONProducesValidJSONWithAllFields(t *testing.T) {
	g := testGraph(t)
	s := testStore(t)
	if _, err := summarizer.ApplySummaries(g, s, summarizer.LevelNode, "test", []summarizer.AppliedSummary{
		{ID: "a", Hash: "h1", Summary: "Package a does things."},
	}); err != nil {
		t.Fatalf("ApplySummaries() error = %v", err)
	}
	if err := s.SetLastCommit("/repos/demo", "abc123"); err != nil {
		t.Fatalf("SetLastCommit() error = %v", err)
	}

	rationale := &graph.Node{
		ID:         graph.RationaleID("f.go", "1", "WHY"),
		Type:       graph.NodeTypeRationale,
		Properties: map[string]any{"kind": "WHY", "text": "Kept simple on purpose."},
	}
	g.AddNode(rationale)
	if err := g.AddEdge(&graph.Edge{
		ID: graph.EdgeID(graph.EdgeTypeExplains, rationale.ID, "a"), Type: graph.EdgeTypeExplains,
		SrcID: rationale.ID, DstID: "a", Confidence: graph.ConfidenceInferred,
	}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}

	// AnalyzeGraph ran against g before the summary was applied to the
	// store, but ApplySummaries doesn't mutate g — re-fetch is unnecessary
	// since ToJSON reads summaries directly from the store.

	out, err := ToJSON(g, s, "/repos/demo")
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	var doc Graph
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if doc.NodeCount != 3 || doc.EdgeCount != 2 {
		t.Fatalf("expected node_count=3 edge_count=2, got %d/%d", doc.NodeCount, doc.EdgeCount)
	}
	if len(doc.Nodes) != 3 || len(doc.Edges) != 2 {
		t.Fatalf("expected 3 nodes and 2 edges in output, got %d/%d", len(doc.Nodes), len(doc.Edges))
	}

	var nodeA *Node
	for i := range doc.Nodes {
		if doc.Nodes[i].ID == "a" {
			nodeA = &doc.Nodes[i]
		}
	}
	if nodeA == nil {
		t.Fatal("expected node 'a' in export")
	}
	if nodeA.Summary != "Package a does things." {
		t.Errorf("expected node 'a' summary to be exported, got %q", nodeA.Summary)
	}
	if _, ok := nodeA.Properties["degree"]; !ok {
		t.Error("expected node 'a' to carry its analytics properties (degree)")
	}
	if len(nodeA.Rationale) != 1 || nodeA.Rationale[0].Text != "Kept simple on purpose." {
		t.Errorf("expected node 'a' to carry its linked rationale, got %+v", nodeA.Rationale)
	}

	if doc.Edges[0].Confidence == "" {
		t.Error("expected edge confidence to be exported")
	}

	if len(doc.Communities) == 0 {
		t.Error("expected at least one community in the export")
	}
	for _, c := range doc.Communities {
		if len(c.NodeIDs) == 0 {
			t.Errorf("expected community %d to list its member node IDs", c.ID)
		}
	}

	if len(doc.GodNodes) == 0 {
		t.Error("expected at least one god node in the export (AnalyzeGraph was called with godNodeCount=1)")
	}

	if doc.Metadata.RepoPath != "/repos/demo" {
		t.Errorf("expected metadata.repo_path = %q, got %q", "/repos/demo", doc.Metadata.RepoPath)
	}
	if doc.Metadata.BuiltAt == "" {
		t.Error("expected metadata.built_at to be populated from build_meta")
	}
	if doc.Metadata.NodeCount != 3 || doc.Metadata.EdgeCount != 2 {
		t.Errorf("expected metadata node/edge counts to match, got %d/%d", doc.Metadata.NodeCount, doc.Metadata.EdgeCount)
	}
}

func TestToJSONMetadataBuiltAtEmptyWhenNoBuildRecorded(t *testing.T) {
	g := testGraph(t)
	s := testStore(t)

	out, err := ToJSON(g, s, "/repos/never-built")
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}
	var doc Graph
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if doc.Metadata.BuiltAt != "" {
		t.Errorf("expected empty built_at for a repo with no recorded build, got %q", doc.Metadata.BuiltAt)
	}
}
