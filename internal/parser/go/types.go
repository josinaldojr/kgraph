package goparser

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/packages"

	"kgraph/internal/graph"
	"kgraph/internal/parser/common"
)

// ormFieldCandidate records a struct field with an ORM tag, deferred for
// the ORM mapping pass.
type ormFieldCandidate struct {
	structID   string
	structName string
	pkgPath    string
	fieldID    string
	fieldName  string
	goTypeName string
	tag        string
}

// extractTypes walks every declared type in every loaded package and
// creates Struct/Interface/Field nodes and has_field edges (pass 2).
func (ex *extractor) extractTypes() {
	ex.forEachFile(func(pkg *packages.Package, file *ast.File) {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch t := typeSpec.Type.(type) {
				case *ast.StructType:
					ex.extractStruct(pkg, typeSpec, t)
				case *ast.InterfaceType:
					ex.extractInterface(pkg, typeSpec, t)
				}
			}
		}
	})
}

func (ex *extractor) extractStruct(pkg *packages.Package, spec *ast.TypeSpec, st *ast.StructType) {
	structID := graph.StructID(pkg.PkgPath, spec.Name.Name)
	pos := ex.fset.Position(spec.Pos())
	end := ex.fset.Position(spec.End())

	ex.g.AddNode(&graph.Node{
		ID:        structID,
		Type:      graph.NodeTypeStruct,
		File:      pos.Filename,
		LineStart: pos.Line,
		LineEnd:   end.Line,
		Signature: "type " + spec.Name.Name + " struct",
		Hash:      graph.ContentHash(printNode(ex.fset, spec)),
		Properties: map[string]any{
			"package":  pkg.PkgPath,
			"name":     spec.Name.Name,
			"language": string(common.LangGo),
		},
	})

	if st.Fields == nil {
		return
	}
	for _, field := range st.Fields.List {
		tag := fieldTag(field)
		typeName, isSlice := baseTypeName(field.Type)
		for _, name := range fieldNames(field) {
			fieldID := graph.FieldID(structID, name)
			fpos := ex.fset.Position(field.Pos())
			ex.g.AddNode(&graph.Node{
				ID:        fieldID,
				Type:      graph.NodeTypeField,
				File:      fpos.Filename,
				LineStart: fpos.Line,
				LineEnd:   fpos.Line,
				Signature: name + " " + printNode(ex.fset, field.Type),
				Hash:      graph.ContentHash(printNode(ex.fset, field)),
				Properties: map[string]any{
					"go_type":  typeName,
					"slice":    isSlice,
					"tag":      tag,
					"language": string(common.LangGo),
				},
			})
			edge := &graph.Edge{
				ID:    graph.EdgeID(graph.EdgeTypeHasField, structID, fieldID),
				Type:  graph.EdgeTypeHasField,
				SrcID: structID,
				DstID: fieldID,
			}
			if err := ex.g.AddEdge(edge); err != nil {
				ex.warnings = append(ex.warnings, err.Error())
			}

			if tag != "" && (structTagHas(tag, "gorm") || structTagHas(tag, "db")) {
				ex.ormFields = append(ex.ormFields, ormFieldCandidate{
					structID:   structID,
					structName: spec.Name.Name,
					pkgPath:    pkg.PkgPath,
					fieldID:    fieldID,
					fieldName:  name,
					goTypeName: typeName,
					tag:        tag,
				})
			}
		}
	}
}

func (ex *extractor) extractInterface(pkg *packages.Package, spec *ast.TypeSpec, it *ast.InterfaceType) {
	ifaceID := graph.InterfaceID(pkg.PkgPath, spec.Name.Name)
	pos := ex.fset.Position(spec.Pos())
	end := ex.fset.Position(spec.End())

	var methods []string
	if it.Methods != nil {
		for _, m := range it.Methods.List {
			for _, name := range fieldNames(m) {
				methods = append(methods, name)
			}
		}
	}

	ex.g.AddNode(&graph.Node{
		ID:        ifaceID,
		Type:      graph.NodeTypeInterface,
		File:      pos.Filename,
		LineStart: pos.Line,
		LineEnd:   end.Line,
		Signature: "type " + spec.Name.Name + " interface",
		Hash:      graph.ContentHash(printNode(ex.fset, spec)),
		Properties: map[string]any{
			"package":  pkg.PkgPath,
			"name":     spec.Name.Name,
			"methods":  methods,
			"language": string(common.LangGo),
		},
	})
}
