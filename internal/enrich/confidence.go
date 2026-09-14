package enrich

import "kgraph/internal/graph"

// inferredEdgeTypes are relationships resolved via cross-file analysis,
// heuristic matching, or framework-specific regex detection rather than
// read directly off explicit syntax — per edge-confidence's provenance
// model and design.md Decision 2.
var inferredEdgeTypes = map[graph.EdgeType]bool{
	graph.EdgeTypeImplements:  true,
	graph.EdgeTypeMapsToTable: true,
	graph.EdgeTypeReadsTable:  true,
	graph.EdgeTypeWritesTable: true,
	graph.EdgeTypeInjected:    true,
	graph.EdgeTypeDecorated:   true,
	graph.EdgeTypeRouted:      true,
	graph.EdgeTypeExplains:    true,
}

// AnnotateConfidence sets Confidence on every edge in g whose Confidence is
// currently unset, based on its edge type: EXTRACTED for relationships
// read directly from source (imports, calls, has_method, has_field,
// embeds, extends, references_fk, exposes_endpoint), INFERRED for ones
// resolved by cross-file or heuristic analysis (implements, maps_to_table,
// reads_table, writes_table, injected, decorated, routed, explains). An
// edge that already carries a Confidence (e.g. one LinkRationale just set)
// is left untouched.
func AnnotateConfidence(g *graph.Graph) {
	for _, e := range g.Edges() {
		if e.Confidence != "" {
			continue
		}
		if inferredEdgeTypes[e.Type] {
			e.Confidence = graph.ConfidenceInferred
		} else {
			e.Confidence = graph.ConfidenceExtracted
		}
	}
}
