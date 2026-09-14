package goparser

import (
	"sort"
	"strings"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/parser/common"
)

// extractPackagesAndImports creates one Package node per loaded package and
// imports edges to either another Package node (import within the module)
// or an ExternalDependency node (everything else).
func (ex *extractor) extractPackagesAndImports() {
	for _, pkg := range ex.pkgs {
		importPaths := make([]string, 0, len(pkg.Imports))
		for path := range pkg.Imports {
			importPaths = append(importPaths, path)
		}
		sort.Strings(importPaths)

		node := &graph.Node{
			ID:   graph.PackageID(pkg.PkgPath),
			Type: graph.NodeTypePackage,
			Properties: map[string]any{
				"name":     pkg.Name,
				"imports":  importPaths,
				"language": string(common.LangGo),
			},
			Hash: graph.ContentHash(pkg.PkgPath + "|" + strings.Join(importPaths, ",")),
		}
		if len(pkg.GoFiles) > 0 {
			node.File = pkg.GoFiles[0]
		}
		ex.g.AddNode(node)
	}

	for _, pkg := range ex.pkgs {
		srcID := graph.PackageID(pkg.PkgPath)
		for path := range pkg.Imports {
			var dstID string
			if ex.internal[path] {
				dstID = graph.PackageID(path)
			} else {
				dstID = graph.ExternalDependencyID(path)
				ex.g.AddNode(&graph.Node{
					ID:   dstID,
					Type: graph.NodeTypeExternalDependency,
					Properties: map[string]any{
						"import_path": path,
						"language":    string(common.LangGo),
					},
					Hash: graph.ContentHash(path),
				})
			}
			edge := &graph.Edge{
				ID:    graph.EdgeID(graph.EdgeTypeImports, srcID, dstID),
				Type:  graph.EdgeTypeImports,
				SrcID: srcID,
				DstID: dstID,
			}
			if err := ex.g.AddEdge(edge); err != nil {
				ex.warnings = append(ex.warnings, err.Error())
			}
		}
	}
}
