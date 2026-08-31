package parser

import (
	"reflect"
	"strings"

	"kgraph/internal/graph"
)

// structTagHas reports whether the given raw struct tag string contains a
// value for key (e.g. "gorm" or "db").
func structTagHas(tag, key string) bool {
	return reflect.StructTag(tag).Get(key) != ""
}

// parseGormTag splits a gorm tag value (e.g. "column:foo;primaryKey") into
// its semicolon-separated sub-options. A flag with no value (e.g.
// "primaryKey") maps to "true".
func parseGormTag(value string) map[string]string {
	out := make(map[string]string)
	for _, part := range strings.Split(value, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i := strings.Index(part, ":"); i >= 0 {
			out[part[:i]] = part[i+1:]
		} else {
			out[part] = "true"
		}
	}
	return out
}

// columnName resolves a field's ORM column name: an explicit `db` tag wins,
// then an explicit gorm `column:` sub-option, falling back to the
// snake_cased Go field name.
func columnName(cand ormFieldCandidate) string {
	tag := reflect.StructTag(cand.tag)
	if db := tag.Get("db"); db != "" {
		return strings.Split(db, ",")[0]
	}
	if gorm := tag.Get("gorm"); gorm != "" {
		if col, ok := parseGormTag(gorm)["column"]; ok && col != "" {
			return col
		}
	}
	return toSnakeCase(cand.fieldName)
}

// mapORMFields turns struct fields carrying gorm/db tags into Table/Column
// nodes and maps_to_table/references_fk edges (pass 2b). It runs after
// extractTypes so every struct in the module is already known, which lets
// the foreign-key heuristics below match association fields against other
// ORM-mapped structs regardless of declaration order.
func (ex *extractor) mapORMFields() {
	if len(ex.ormFields) == 0 {
		return
	}

	structTable := make(map[string]string)   // structID -> table name
	typeNameTable := make(map[string]string) // bare struct type name -> table name (best-effort, last-write-wins across packages)

	for _, cand := range ex.ormFields {
		table, ok := structTable[cand.structID]
		if !ok {
			table = pluralize(toSnakeCase(cand.structName))
			structTable[cand.structID] = table
			typeNameTable[cand.structName] = table

			ex.g.AddNode(&graph.Node{
				ID:         graph.TableID(table),
				Type:       graph.NodeTypeTable,
				Properties: map[string]any{"name": table, "source": "struct_tag"},
				Hash:       graph.ContentHash("struct_tag:" + table),
			})
			edge := &graph.Edge{
				ID:    graph.EdgeID(graph.EdgeTypeMapsToTable, cand.structID, graph.TableID(table)),
				Type:  graph.EdgeTypeMapsToTable,
				SrcID: cand.structID,
				DstID: graph.TableID(table),
			}
			if err := ex.g.AddEdge(edge); err != nil {
				ex.warnings = append(ex.warnings, err.Error())
			}
		}

		col := columnName(cand)
		colID := graph.ColumnID(table, col)
		ex.g.AddNode(&graph.Node{
			ID:   colID,
			Type: graph.NodeTypeColumn,
			Properties: map[string]any{
				"name":     col,
				"go_field": cand.fieldName,
				"go_type":  cand.goTypeName,
				"source":   "struct_tag",
			},
			Hash: graph.ContentHash("struct_tag:" + colID + ":" + cand.tag),
		})
		colEdge := &graph.Edge{
			ID:    graph.EdgeID(graph.EdgeTypeHasField, graph.TableID(table), colID),
			Type:  graph.EdgeTypeHasField,
			SrcID: graph.TableID(table),
			DstID: colID,
		}
		if err := ex.g.AddEdge(colEdge); err != nil {
			ex.warnings = append(ex.warnings, err.Error())
		}
	}

	// Foreign-key heuristics, once every struct's table name is known.
	for _, cand := range ex.ormFields {
		table := structTable[cand.structID]
		colID := graph.ColumnID(table, columnName(cand))

		// Heuristic 1: the field's own Go type is another mapped struct
		// (an association field, e.g. `Owner *User` or `Items []Item`).
		if relTable, ok := typeNameTable[cand.goTypeName]; ok && relTable != table {
			ex.addFKEdge(graph.TableID(table), graph.TableID(relTable))
			continue
		}

		// Heuristic 2: the field name follows the "<Name>ID" convention
		// (e.g. `UserID` referencing struct `User`).
		if strings.HasSuffix(cand.fieldName, "ID") && cand.fieldName != "ID" {
			base := strings.TrimSuffix(cand.fieldName, "ID")
			if relTable, ok := typeNameTable[base]; ok {
				ex.addFKEdge(colID, graph.TableID(relTable))
			}
		}
	}
}

func (ex *extractor) addFKEdge(srcID, dstID string) {
	edge := &graph.Edge{
		ID:    graph.EdgeID(graph.EdgeTypeReferencesFK, srcID, dstID),
		Type:  graph.EdgeTypeReferencesFK,
		SrcID: srcID,
		DstID: dstID,
	}
	if err := ex.g.AddEdge(edge); err != nil {
		ex.warnings = append(ex.warnings, err.Error())
	}
}
