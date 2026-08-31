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
