package goparser

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"

	"kgraph/internal/graph"
)

// extractCalls walks every function body and adds a calls edge for each
// call expression that resolves to another Function node already in the graph.
func (ex *extractor) extractCalls() {
	ex.forEachFile(func(pkg *packages.Package, file *ast.File) {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			callerID := graph.FunctionID(pkg.PkgPath, receiverTypeName(fd), fd.Name.Name)

			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				calleeID, ok := ex.resolveCallee(pkg, call)
				if !ok || ex.g.Node(calleeID) == nil {
					return true
				}
				edge := &graph.Edge{
					ID:    graph.EdgeID(graph.EdgeTypeCalls, callerID, calleeID),
					Type:  graph.EdgeTypeCalls,
					SrcID: callerID,
					DstID: calleeID,
				}
				if err := ex.g.AddEdge(edge); err != nil {
					ex.warnings = append(ex.warnings, err.Error())
				}
				return true
			})
		}
	})
}

// resolveCallee resolves a call expression's target function to a
// deterministic Function node ID.
func (ex *extractor) resolveCallee(pkg *packages.Package, call *ast.CallExpr) (string, bool) {
	if pkg.TypesInfo == nil {
		return "", false
	}

	var ident *ast.Ident
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		ident = fn
	case *ast.SelectorExpr:
		ident = fn.Sel
	default:
		return "", false
	}

	obj := pkg.TypesInfo.Uses[ident]
	fnObj, ok := obj.(*types.Func)
	if !ok || fnObj.Pkg() == nil {
		return "", false
	}
	calleePkgPath := fnObj.Pkg().Path()
	if !ex.internal[calleePkgPath] {
		return "", false
	}

	receiver := ""
	if sig, ok := fnObj.Type().(*types.Signature); ok && sig.Recv() != nil {
		receiver = namedTypeName(sig.Recv().Type())
	}
	return graph.FunctionID(calleePkgPath, receiver, fnObj.Name()), true
}

// namedTypeName strips a pointer wrapper and returns a named type's bare
// identifier.
func namedTypeName(t types.Type) string {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}
