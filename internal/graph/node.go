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
	NodeTypeEnum               NodeType = "Enum"
	NodeTypeDecorator          NodeType = "Decorator"
	NodeTypeVariable           NodeType = "Variable"
	NodeTypeTypeAlias          NodeType = "TypeAlias"
	NodeTypeRationale          NodeType = "Rationale"
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

// Property keys used by the analytics/enrich stages to store metadata in
// Node.Properties. Kept here (rather than typed struct fields) so analytics
// metadata doesn't require a schema migration — see design.md Decision 1.
const (
	PropertyDegree         = "degree"
	PropertyGodNode        = "god_node"
	PropertyCommunity      = "community"
	PropertyCommunityLabel = "community_label"
	PropertyRationaleCount = "rationale_count"
)

// Community returns the node's community ID, or -1 if it hasn't been
// assigned one (e.g. analytics hasn't run yet).
func (n *Node) Community() int {
	v, ok := n.Properties[PropertyCommunity]
	if !ok {
		return -1
	}
	switch c := v.(type) {
	case int:
		return c
	case float64:
		return int(c)
	default:
		return -1
	}
}

// IsGodNode reports whether the node was flagged as a god node (top-N by
// degree) during the analyze stage.
func (n *Node) IsGodNode() bool {
	v, ok := n.Properties[PropertyGodNode]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// Degree returns the node's precomputed total degree (in + out edges), or 0
// if analytics hasn't run yet.
func (n *Node) Degree() int {
	v, ok := n.Properties[PropertyDegree]
	if !ok {
		return 0
	}
	switch d := v.(type) {
	case int:
		return d
	case float64:
		return int(d)
	default:
		return 0
	}
}

// CommunityLabel returns the human-readable label derived for the node's
// community, or "" if none has been assigned.
func (n *Node) CommunityLabel() string {
	v, ok := n.Properties[PropertyCommunityLabel]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
