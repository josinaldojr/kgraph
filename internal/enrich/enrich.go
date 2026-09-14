// Package enrich adds semantic metadata to an already-parsed graph:
// rationale extraction (NOTE/WHY/HACK comments and docstrings as first-class
// nodes) and edge confidence tagging (EXTRACTED vs INFERRED). It runs as
// the Enrich stage between Parse and Analyze in the build pipeline, per
// design.md Decision 3.
package enrich

import (
	"fmt"
	"os"
	"sort"

	"github.com/josinaldojr/kgraph/internal/graph"
)

// EnrichGraph runs rationale extraction and confidence annotation over
// every file represented in g, mutating it in place: new Rationale nodes
// and `explains` edges are added, and every edge's Confidence is filled in.
// It returns non-fatal warnings (e.g. a source file that no longer exists
// on disk) rather than failing the whole pass.
func EnrichGraph(g *graph.Graph) ([]string, error) {
	var warnings []string

	byFile := make(map[string][]*graph.Node)
	for _, n := range g.Nodes() {
		if n.File == "" || n.Type == graph.NodeTypeRationale {
			continue
		}
		byFile[n.File] = append(byFile[n.File], n)
	}

	for file, entities := range byFile {
		content, err := os.ReadFile(file)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("enrich: reading %s: %v", file, err))
			continue
		}
		text := string(content)

		rationale := ExtractRationale(g, file, text)
		rationale = append(rationale, ExtractDocstrings(g, file, text)...)
		if len(rationale) == 0 {
			continue
		}

		sort.Slice(entities, func(i, j int) bool { return entities[i].LineStart < entities[j].LineStart })

		byTarget := make(map[string][]*graph.Node)
		var order []string
		for _, r := range rationale {
			g.AddNode(r)
			target := nearestEntity(entities, r.LineEnd)
			if target == nil {
				continue
			}
			if _, ok := byTarget[target.ID]; !ok {
				order = append(order, target.ID)
			}
			byTarget[target.ID] = append(byTarget[target.ID], r)
		}
		for _, tid := range order {
			LinkRationale(g, byTarget[tid], g.Node(tid))
		}
	}

	AnnotateConfidence(g)
	return warnings, nil
}

// nearestEntity returns the code entity (from entities, sorted by
// LineStart ascending) that a comment ending at line most plausibly
// documents: the first entity starting at or after the comment, falling
// back to the last entity starting before it (the enclosing declaration),
// or nil if entities is empty. entities' sortedness lets this binary
// search rather than scan.
func nearestEntity(entities []*graph.Node, line int) *graph.Node {
	if len(entities) == 0 {
		return nil
	}
	idx := sort.Search(len(entities), func(i int) bool { return entities[i].LineStart >= line })
	if idx == len(entities) {
		return entities[len(entities)-1]
	}
	return entities[idx]
}
