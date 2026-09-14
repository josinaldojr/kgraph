package goparser

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"
	"unicode"
)

// printNode renders an AST node back to source text using the given
// FileSet.
func printNode(fset *token.FileSet, n ast.Node) string {
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
	if err := cfg.Fprint(&buf, fset, n); err != nil {
		return ""
	}
	return buf.String()
}

// receiverTypeName returns the (unqualified, pointer-stripped) receiver
// type name of a method declaration, or "" for a plain function.
func receiverTypeName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	expr := fd.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// fieldNames returns the declared names for a struct/interface field.
func fieldNames(field *ast.Field) []string {
	if len(field.Names) > 0 {
		names := make([]string, 0, len(field.Names))
		for _, n := range field.Names {
			names = append(names, n.Name)
		}
		return names
	}
	return []string{embeddedFieldName(field.Type)}
}

// embeddedFieldName extracts the type name used as an embedded field's
// implicit name.
func embeddedFieldName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return embeddedFieldName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	default:
		return ""
	}
}

// fieldTag returns the raw (unquoted) struct tag text for a field, or ""
// if it has none.
func fieldTag(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}
	return strings.Trim(field.Tag.Value, "`")
}

// toSnakeCase converts a Go identifier to snake_case.
func toSnakeCase(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLower(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return strings.TrimPrefix(b.String(), "_")
}

// pluralize applies a naive English pluralization.
func pluralize(snake string) string {
	switch {
	case strings.HasSuffix(snake, "y") && len(snake) > 1 && !isVowel(rune(snake[len(snake)-2])):
		return snake[:len(snake)-1] + "ies"
	case strings.HasSuffix(snake, "s"), strings.HasSuffix(snake, "sh"),
		strings.HasSuffix(snake, "ch"), strings.HasSuffix(snake, "x"):
		return snake + "es"
	default:
		return snake + "s"
	}
}

func isVowel(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// baseTypeName strips pointer and slice wrappers to get the underlying
// named type's identifier.
func baseTypeName(expr ast.Expr) (name string, isSlice bool) {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return baseTypeName(t.X)
	case *ast.ArrayType:
		n, _ := baseTypeName(t.Elt)
		return n, true
	case *ast.Ident:
		return t.Name, false
	case *ast.SelectorExpr:
		return t.Sel.Name, false
	default:
		return "", false
	}
}
