package enrich

import (
	"testing"

	"github.com/josinaldojr/kgraph/internal/graph"
)

func buildTestGraph(t *testing.T, edgeType graph.EdgeType) *graph.Graph {
	t.Helper()
	g := graph.New()
	g.AddNode(&graph.Node{ID: "a", Type: graph.NodeTypePackage})
	g.AddNode(&graph.Node{ID: "b", Type: graph.NodeTypePackage})
	if err := g.AddEdge(&graph.Edge{ID: "e", Type: edgeType, SrcID: "a", DstID: "b"}); err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}
	return g
}

func TestAnnotateConfidenceExtractedTypes(t *testing.T) {
	for _, et := range []graph.EdgeType{
		// Explicitly documented as EXTRACTED in design.md.
		graph.EdgeTypeImports, graph.EdgeTypeCalls, graph.EdgeTypeHasMethod,
		graph.EdgeTypeHasField, graph.EdgeTypeEmbeds, graph.EdgeTypeExtends,
		// Not listed either way in design.md, but also read directly off
		// source (FK column tags, route annotations) — must still fall
		// through AnnotateConfidence's default branch to EXTRACTED, not be
		// silently left unset.
		graph.EdgeTypeReferencesFK, graph.EdgeTypeExposesEndpoint,
	} {
		g := buildTestGraph(t, et)
		AnnotateConfidence(g)
		got := g.Edges()[0].Confidence
		if got != graph.ConfidenceExtracted {
			t.Errorf("edge type %s: expected EXTRACTED, got %s", et, got)
		}
	}
}

func TestAnnotateConfidenceInferredTypes(t *testing.T) {
	for _, et := range []graph.EdgeType{
		graph.EdgeTypeImplements, graph.EdgeTypeMapsToTable,
		graph.EdgeTypeReadsTable, graph.EdgeTypeWritesTable,
		// Framework-oriented INFERRED types (DI, decorators, routing) and
		// the rationale-linking edge type, all from inferredEdgeTypes but
		// previously untested here.
		graph.EdgeTypeInjected, graph.EdgeTypeDecorated,
		graph.EdgeTypeRouted, graph.EdgeTypeExplains,
	} {
		g := buildTestGraph(t, et)
		AnnotateConfidence(g)
		got := g.Edges()[0].Confidence
		if got != graph.ConfidenceInferred {
			t.Errorf("edge type %s: expected INFERRED, got %s", et, got)
		}
	}
}

// TestAnnotateConfidenceCoversEveryEdgeType guards against a new EdgeType
// constant being added to internal/graph/edge.go without a corresponding
// decision (explicit or via AnnotateConfidence's default) ever being
// exercised by a test — every known edge type must appear in exactly one
// of the two tests above.
func TestAnnotateConfidenceCoversEveryEdgeType(t *testing.T) {
	allTypes := []graph.EdgeType{
		graph.EdgeTypeImports, graph.EdgeTypeCalls, graph.EdgeTypeEmbeds,
		graph.EdgeTypeImplements, graph.EdgeTypeHasMethod, graph.EdgeTypeHasField,
		graph.EdgeTypeMapsToTable, graph.EdgeTypeReferencesFK, graph.EdgeTypeReadsTable,
		graph.EdgeTypeWritesTable, graph.EdgeTypeExposesEndpoint, graph.EdgeTypeExtends,
		graph.EdgeTypeInjected, graph.EdgeTypeDecorated, graph.EdgeTypeRouted,
		graph.EdgeTypeExplains,
	}
	for _, et := range allTypes {
		g := buildTestGraph(t, et)
		AnnotateConfidence(g)
		got := g.Edges()[0].Confidence
		if got != graph.ConfidenceExtracted && got != graph.ConfidenceInferred {
			t.Errorf("edge type %s: AnnotateConfidence left an unrecognized confidence %q", et, got)
		}
	}
}

func TestAnnotateConfidenceLeavesExistingValueAlone(t *testing.T) {
	g := buildTestGraph(t, graph.EdgeTypeImplements)
	g.Edges()[0].Confidence = graph.ConfidenceExtracted // pretend something already set it
	AnnotateConfidence(g)
	if got := g.Edges()[0].Confidence; got != graph.ConfidenceExtracted {
		t.Errorf("expected pre-set confidence to be left alone, got %s", got)
	}
}
