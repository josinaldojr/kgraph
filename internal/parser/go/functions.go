package goparser

import (
	"go/ast"

	"golang.org/x/tools/go/packages"

	"kgraph/internal/graph"
	"kgraph/internal/parser/common"
)

// extractFuncs walks every function/method declaration in every loaded
// package and creates Function nodes plus has_method edges to their
// receiver's Struct node (pass 3a).
func (ex *extractor) extractFuncs() {
	ex.forEachFile(func(pkg *packages.Package, file *ast.File) {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ex.extractFunc(pkg, fd)
		}
	})
}

func (ex *extractor) extractFunc(pkg *packages.Package, fd *ast.FuncDecl) {
	receiver := receiverTypeName(fd)
	funcID := graph.FunctionID(pkg.PkgPath, receiver, fd.Name.Name)
	pos := ex.fset.Position(fd.Pos())
	end := ex.fset.Position(fd.End())

	sigDecl := &ast.FuncDecl{Recv: fd.Recv, Name: fd.Name, Type: fd.Type}

	ex.g.AddNode(&graph.Node{
		ID:        funcID,
		Type:      graph.NodeTypeFunction,
		File:      pos.Filename,
		LineStart: pos.Line,
		LineEnd:   end.Line,
		Signature: printNode(ex.fset, sigDecl),
		Hash:      graph.ContentHash(printNode(ex.fset, fd)),
		Properties: map[string]any{
			"package":  pkg.PkgPath,
			"name":     fd.Name.Name,
			"receiver": receiver,
			"exported": fd.Name.IsExported(),
			"language": string(common.LangGo),
		},
	})

	if receiver == "" {
		return
	}
	structID := graph.StructID(pkg.PkgPath, receiver)
	if ex.g.Node(structID) == nil {
		return
	}
	edge := &graph.Edge{
		ID:    graph.EdgeID(graph.EdgeTypeHasMethod, structID, funcID),
		Type:  graph.EdgeTypeHasMethod,
		SrcID: structID,
		DstID: funcID,
	}
	if err := ex.g.AddEdge(edge); err != nil {
		ex.warnings = append(ex.warnings, err.Error())
	}
}
