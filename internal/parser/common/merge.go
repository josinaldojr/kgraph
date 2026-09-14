package common

import (
	"fmt"
	"strings"

	"github.com/josinaldojr/kgraph/internal/graph"
)

// MergeGraphs combines multiple graphs into one unified graph.
// Nodes from later graphs overwrite nodes from earlier graphs if IDs conflict.
// Edges are added best-effort.
func MergeGraphs(graphs ...*graph.Graph) *graph.Graph {
	merged := graph.New()

	for _, g := range graphs {
		if g == nil {
			continue
		}

		// Add all nodes
		for _, n := range g.Nodes() {
			merged.AddNode(n)
		}

		// Add all edges
		for _, e := range g.Edges() {
			_ = merged.AddEdge(e)
		}
	}

	return merged
}

// MergeGraphsWithWarnings combines multiple graphs with conflict detection.
// Returns the merged graph and any warnings about ID conflicts.
func MergeGraphsWithWarnings(graphs ...*graph.Graph) (*graph.Graph, []string) {
	merged := graph.New()
	var warnings []string
	seenNodes := make(map[string]string) // nodeID -> source graph index

	for i, g := range graphs {
		if g == nil {
			continue
		}

		// Add all nodes
		for _, n := range g.Nodes() {
			if existingSource, exists := seenNodes[n.ID]; exists {
				warnings = append(warnings, fmt.Sprintf(
					"node ID conflict: %s already exists from graph %s, overwriting with graph %d",
					n.ID, existingSource, i,
				))
			}
			seenNodes[n.ID] = fmt.Sprintf("%d", i)
			merged.AddNode(n)
		}

		// Add all edges
		for _, e := range g.Edges() {
			if err := merged.AddEdge(e); err != nil {
				warnings = append(warnings, fmt.Sprintf("edge conflict: %v", err))
			}
		}
	}

	return merged, warnings
}

// KnownInternalForLanguage returns the import paths of every Package node
// belonging to lang in g, as a set — used to scope an incremental
// ExtractPackages call to only the packages a given language's extractor
// already produced in a prior build. Without this, a single flat
// knownInternal map built from the whole (multi-language) graph would hand
// every extractor package names from every other language too (audit
// 2026-09-13; see incremental-update spec's "Known Internal Packages").
func KnownInternalForLanguage(g *graph.Graph, lang Language) map[string]bool {
	out := make(map[string]bool)
	if g == nil {
		return out
	}
	for _, n := range g.Nodes() {
		if n.Type != graph.NodeTypePackage {
			continue
		}
		if langProp, _ := n.Properties["language"].(string); langProp == string(lang) {
			out[strings.TrimPrefix(n.ID, "pkg:")] = true
		}
	}
	return out
}
