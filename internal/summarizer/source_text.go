package summarizer

import (
	"fmt"
	"sort"
	"strings"

	"kgraph/internal/graph"
)

// sourceTextFor builds a pending-summary's "source text" for a node type
// with no Go source span of its own (Table, Endpoint, ExternalDependency),
// per entity-summarization's "Pending-summary export" requirement: each
// type gets a type-appropriate rendering instead of raw source.
func sourceTextFor(g *graph.Graph, n *graph.Node) string {
	switch n.Type {
	case graph.NodeTypeTable:
		return tableSourceText(g, n)
	case graph.NodeTypeEndpoint:
		return endpointSourceText(g, n)
	case graph.NodeTypeExternalDependency:
		return externalDependencySourceText(n)
	default:
		return ""
	}
}

// tableSourceText renders a Table's column list (via its has_field edges)
// plus any incoming references_fk edges (other tables/columns that point
// at this one as a foreign key target).
func tableSourceText(g *graph.Graph, table *graph.Node) string {
	var cols []string
	for _, e := range g.OutEdges(table.ID) {
		if e.Type != graph.EdgeTypeHasField {
			continue
		}
		col := g.Node(e.DstID)
		if col == nil {
			continue
		}
		name, _ := col.Properties["name"].(string)
		if name == "" {
			name = col.ID
		}
		colType, _ := col.Properties["sql_type"].(string)
		if colType == "" {
			colType, _ = col.Properties["go_type"].(string)
		}
		if colType != "" {
			cols = append(cols, fmt.Sprintf("%s %s", name, colType))
		} else {
			cols = append(cols, name)
		}
	}
	sort.Strings(cols)

	var refs []string
	for _, e := range g.InEdges(table.ID) {
		if e.Type != graph.EdgeTypeReferencesFK {
			continue
		}
		refs = append(refs, e.SrcID)
	}
	sort.Strings(refs)

	tableName, _ := table.Properties["name"].(string)
	if tableName == "" {
		tableName = strings.TrimPrefix(table.ID, "table:")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "TABLE %s\n", tableName)
	if len(cols) > 0 {
		fmt.Fprintf(&b, "Columns:\n")
		for _, c := range cols {
			fmt.Fprintf(&b, "  - %s\n", c)
		}
	}
	if len(refs) > 0 {
		fmt.Fprintf(&b, "Referenced by (foreign key): %s\n", strings.Join(refs, ", "))
	}
	return b.String()
}

// endpointSourceText renders an Endpoint's HTTP method/path (decoded from
// its deterministic "endpoint:METHOD PATH" ID, per graph.EndpointID) plus
// its handler function's signature, found via the incoming exposes_endpoint
// edge (handler --exposes_endpoint--> endpoint).
func endpointSourceText(g *graph.Graph, endpoint *graph.Node) string {
	methodAndPath := strings.TrimPrefix(endpoint.ID, "endpoint:")

	var handler string
	for _, e := range g.InEdges(endpoint.ID) {
		if e.Type != graph.EdgeTypeExposesEndpoint {
			continue
		}
		if h := g.Node(e.SrcID); h != nil {
			handler = h.Signature
			if handler == "" {
				handler = h.ID
			}
			break
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "ENDPOINT %s\n", methodAndPath)
	if handler != "" {
		fmt.Fprintf(&b, "Handler: %s\n", handler)
	}
	return b.String()
}

// externalDependencySourceText renders an ExternalDependency's module
// (import) path and, when known, its declared version — the extractor does
// not always record a version, so this degrades gracefully to just the
// import path.
func externalDependencySourceText(dep *graph.Node) string {
	path, _ := dep.Properties["import_path"].(string)
	if path == "" {
		path = strings.TrimPrefix(dep.ID, "ext:")
	}
	version, _ := dep.Properties["version"].(string)

	if version != "" {
		return fmt.Sprintf("EXTERNAL DEPENDENCY %s @ %s\n", path, version)
	}
	return fmt.Sprintf("EXTERNAL DEPENDENCY %s\n", path)
}
