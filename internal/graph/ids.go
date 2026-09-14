package graph

import "fmt"

// Deterministic node ID generation, one scheme per node type, so rebuilds
// are idempotent: the same source entity always produces the same ID.

// PackageID returns the ID for a Package node given its import path.
func PackageID(importPath string) string {
	return "pkg:" + importPath
}

// StructID returns the ID for a Struct node given its package import path
// and type name.
func StructID(importPath, name string) string {
	return fmt.Sprintf("%s.%s", importPath, name)
}

// InterfaceID returns the ID for an Interface node given its package
// import path and type name.
func InterfaceID(importPath, name string) string {
	return fmt.Sprintf("%s.%s", importPath, name)
}

// FunctionID returns the ID for a Function node. For a method, receiver is
// the receiver type's name; for a plain function, receiver is empty.
func FunctionID(importPath, receiver, name string) string {
	if receiver == "" {
		return fmt.Sprintf("%s.%s()", importPath, name)
	}
	return fmt.Sprintf("%s.%s.%s()", importPath, receiver, name)
}

// FieldID returns the ID for a Field node owned by a Struct or Interface.
func FieldID(ownerID, fieldName string) string {
	return fmt.Sprintf("%s#%s", ownerID, fieldName)
}

// TableID returns the ID for a Table node given its table name.
func TableID(tableName string) string {
	return "table:" + tableName
}

// ColumnID returns the ID for a Column node given its table name and
// column name.
func ColumnID(tableName, columnName string) string {
	return fmt.Sprintf("table:%s.%s", tableName, columnName)
}

// EndpointID returns the ID for an Endpoint node given its HTTP method and
// path.
func EndpointID(method, path string) string {
	return fmt.Sprintf("endpoint:%s %s", method, path)
}

// ExternalDependencyID returns the ID for an ExternalDependency node given
// its module/import path.
func ExternalDependencyID(importPath string) string {
	return "ext:" + importPath
}

// EdgeID returns a deterministic ID for an edge, derived from its type and
// endpoints, so re-adding the same logical edge does not create a
// duplicate.
func EdgeID(edgeType EdgeType, srcID, dstID string) string {
	return fmt.Sprintf("%s|%s|%s", edgeType, srcID, dstID)
}

// EnumID returns the ID for an Enum node given its package import path
// and type name. Prefixed (like DecoratorID) because, unlike Go where
// top-level identifiers are unique per package, Java/TS/Python don't
// guarantee an Enum name can't collide with a Struct/Variable/TypeAlias
// of the same name in the same package.
func EnumID(importPath, name string) string {
	return fmt.Sprintf("enum:%s.%s", importPath, name)
}

// DecoratorID returns the ID for a Decorator node given its package
// import path and decorator name.
func DecoratorID(importPath, name string) string {
	return fmt.Sprintf("decorator:%s.%s", importPath, name)
}

// VariableID returns the ID for a Variable node given its package
// import path and variable name. Prefixed for the same reason as EnumID:
// a module-level variable can share a name with a function or class in
// Python/JavaScript in a way Go's compiler would never allow.
func VariableID(importPath, name string) string {
	return fmt.Sprintf("variable:%s.%s", importPath, name)
}

// TypeAliasID returns the ID for a TypeAlias node given its package
// import path and type name. Prefixed for the same reason as EnumID.
func TypeAliasID(importPath, name string) string {
	return fmt.Sprintf("typealias:%s.%s", importPath, name)
}

// RationaleID returns the ID for a Rationale node given the file it was
// extracted from, the line it starts on, and its kind (e.g. "NOTE", "WHY",
// "docstring").
func RationaleID(file, line, kind string) string {
	return fmt.Sprintf("rationale:%s:%s:%s", file, line, kind)
}
