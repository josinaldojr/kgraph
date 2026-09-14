package graph

// EdgeType identifies the kind of relationship an Edge represents.
type EdgeType string

const (
	EdgeTypeImports         EdgeType = "imports"
	EdgeTypeCalls           EdgeType = "calls"
	EdgeTypeEmbeds          EdgeType = "embeds"
	EdgeTypeImplements      EdgeType = "implements"
	EdgeTypeHasMethod       EdgeType = "has_method"
	EdgeTypeHasField        EdgeType = "has_field"
	EdgeTypeMapsToTable     EdgeType = "maps_to_table"
	EdgeTypeReferencesFK    EdgeType = "references_fk"
	EdgeTypeReadsTable      EdgeType = "reads_table"
	EdgeTypeWritesTable     EdgeType = "writes_table"
	EdgeTypeExposesEndpoint EdgeType = "exposes_endpoint"
	EdgeTypeExtends         EdgeType = "extends"
	EdgeTypeInjected        EdgeType = "injected"
	EdgeTypeDecorated       EdgeType = "decorated"
	EdgeTypeRouted          EdgeType = "routed"
	EdgeTypeExplains        EdgeType = "explains"
)

// Confidence describes an edge's provenance: whether it was read directly
// off the AST or inferred via cross-file/heuristic resolution.
const (
	ConfidenceExtracted = "EXTRACTED"
	ConfidenceInferred  = "INFERRED"
)

// Edge is a directed, typed relationship between two nodes.
type Edge struct {
	ID         string
	Type       EdgeType
	SrcID      string
	DstID      string
	Confidence string
	Properties map[string]any
}
