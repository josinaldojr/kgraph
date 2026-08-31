package graph

// NodeType identifies the kind of entity a Node represents.
type NodeType string

const (
	NodeTypePackage            NodeType = "Package"
	NodeTypeStruct             NodeType = "Struct"
	NodeTypeInterface          NodeType = "Interface"
	NodeTypeFunction           NodeType = "Function"
	NodeTypeField              NodeType = "Field"
	NodeTypeTable              NodeType = "Table"
	NodeTypeColumn             NodeType = "Column"
	NodeTypeEndpoint           NodeType = "Endpoint"
	NodeTypeExternalDependency NodeType = "ExternalDependency"
)

// Node is a single entity in the code knowledge graph.
type Node struct {
	ID         string
	Type       NodeType
	File       string
	LineStart  int
	LineEnd    int
	Signature  string
	Hash       string
	Properties map[string]any
}
