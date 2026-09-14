package context

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/summarizer"
)

// resolveTarget finds the node GetContext's target string refers to: an
// exact node ID, a file path (full or basename), or a struct/interface/
// function name — falling back to lexical SearchNodes for anything else
// (a fuzzy/topic query), per context-assembly's "Target resolution"
// requirement.
func resolveTarget(g *graph.Graph, s summarizer.SummaryReader, target string) (*graph.Node, error) {
	if n := g.Node(target); n != nil {
		return n, nil
	}

	var byFile, byName []*graph.Node
	for _, n := range g.Nodes() {
		if n.File != "" && (n.File == target || filepath.Base(n.File) == target) {
			byFile = append(byFile, n)
		}
		if name, _ := n.Properties["name"].(string); name != "" && name == target {
			byName = append(byName, n)
		}
	}

	if len(byName) > 0 {
		sort.Slice(byName, func(i, j int) bool { return byName[i].ID < byName[j].ID })
		return byName[0], nil
	}
	if len(byFile) > 0 {
		// A file target with no single struct/function match resolves to
		// its Package node so callers still get *something* to expand
		// from, preferring the package that owns the most nodes in it.
		if pkgPath, _ := byFile[0].Properties["package"].(string); pkgPath != "" {
			if pkgNode := g.Node(graph.PackageID(pkgPath)); pkgNode != nil {
				return pkgNode, nil
			}
		}
		sort.Slice(byFile, func(i, j int) bool { return byFile[i].ID < byFile[j].ID })
		return byFile[0], nil
	}

	results, err := summarizer.SearchNodes(g, s, target, 1)
	if err != nil {
		return nil, fmt.Errorf("context: resolving target %q via search: %w", target, err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("context: no node found matching %q", target)
	}
	return results[0].Node, nil
}
