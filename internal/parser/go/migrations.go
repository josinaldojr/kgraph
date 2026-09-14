package goparser

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/parser/common"
)

var (
	createTableRe = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?[` + "`" + `"']?(\w+)[` + "`" + `"']?\s*\(`)
	alterAddColRe = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+[` + "`" + `"']?(\w+)[` + "`" + `"']?\s+ADD\s+(?:COLUMN\s+)?[` + "`" + `"']?(\w+)[` + "`" + `"']?\s+([A-Za-z0-9_()]+)`)
	referencesRe  = regexp.MustCompile(`(?i)REFERENCES\s+[` + "`" + `"']?(\w+)[` + "`" + `"']?`)
	tableLevelKw  = regexp.MustCompile(`(?i)^(PRIMARY\s+KEY|UNIQUE|CONSTRAINT|CHECK|KEY|INDEX)\b`)
	fkKw          = regexp.MustCompile(`(?i)^FOREIGN\s+KEY`)
)

// ExtractMigrations parses every .sql file under repoPath for CREATE TABLE
// and ALTER TABLE ADD COLUMN statements.
func ExtractMigrations(repoPath string, g *graph.Graph) ([]string, error) {
	var warnings []string

	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".sql") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", path, readErr))
			return nil
		}
		extractMigrationFile(g, path, string(content))
		return nil
	})
	if err != nil {
		return warnings, fmt.Errorf("parser: walking %s for migrations: %w", repoPath, err)
	}
	return warnings, nil
}

func extractMigrationFile(g *graph.Graph, path, content string) {
	for _, m := range createTableRe.FindAllStringSubmatchIndex(content, -1) {
		tableName := content[m[2]:m[3]]
		openParen := m[1] - 1
		block, ok := matchingParenBlock(content, openParen)
		if !ok {
			continue
		}
		upsertMigrationTable(g, path, tableName)
		for _, def := range splitTopLevel(block) {
			def = strings.TrimSpace(def)
			if def == "" {
				continue
			}
			if fkKw.MatchString(def) || tableLevelKw.MatchString(def) {
				if ref := referencesRe.FindStringSubmatch(def); ref != nil {
					linkTableFK(g, tableName, ref[1])
				}
				continue
			}
			fields := strings.Fields(def)
			if len(fields) < 2 {
				continue
			}
			colName := strings.Trim(fields[0], "`\"'")
			colType := fields[1]
			upsertMigrationColumn(g, tableName, colName, colType, path)
			if ref := referencesRe.FindStringSubmatch(def); ref != nil {
				linkColumnFK(g, tableName, colName, ref[1])
			}
		}
	}

	for _, m := range alterAddColRe.FindAllStringSubmatch(content, -1) {
		tableName, colName, colType := m[1], m[2], m[3]
		upsertMigrationTable(g, path, tableName)
		upsertMigrationColumn(g, tableName, colName, colType, path)
	}
}

func matchingParenBlock(s string, openIdx int) (string, bool) {
	depth := 0
	for i := openIdx; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[openIdx+1 : i], true
			}
		}
	}
	return "", false
}

func splitTopLevel(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}

func upsertMigrationTable(g *graph.Graph, path, tableName string) {
	id := graph.TableID(tableName)
	existing := g.Node(id)
	source := "migration"
	if existing != nil {
		if s, _ := existing.Properties["source"].(string); s == "struct_tag" {
			source = "migration+struct_tag"
		}
	}
	g.AddNode(&graph.Node{
		ID:        id,
		Type:      graph.NodeTypeTable,
		File:      path,
		Signature: "TABLE " + tableName,
		Hash:      graph.ContentHash("migration:" + tableName + ":" + path),
		Properties: map[string]any{
			"name":     tableName,
			"source":   source,
			"language": string(common.LangGo),
		},
	})
}

func upsertMigrationColumn(g *graph.Graph, tableName, colName, colType, path string) {
	id := graph.ColumnID(tableName, colName)
	props := map[string]any{
		"name":     colName,
		"sql_type": colType,
		"source":   "migration",
		"language": string(common.LangGo),
	}

	if existing := g.Node(id); existing != nil {
		if s, _ := existing.Properties["source"].(string); s == "struct_tag" {
			props["struct_tag_conflict"] = existing.Properties
		}
	}

	g.AddNode(&graph.Node{
		ID:         id,
		Type:       graph.NodeTypeColumn,
		File:       path,
		Signature:  colName + " " + colType,
		Hash:       graph.ContentHash("migration:" + id + ":" + colType),
		Properties: props,
	})

	tableID := graph.TableID(tableName)
	if g.Node(tableID) != nil {
		edge := &graph.Edge{
			ID:    graph.EdgeID(graph.EdgeTypeHasField, tableID, id),
			Type:  graph.EdgeTypeHasField,
			SrcID: tableID,
			DstID: id,
		}
		_ = g.AddEdge(edge)
	}
}

func linkTableFK(g *graph.Graph, tableName, refTable string) {
	linkFK(g, graph.TableID(tableName), graph.TableID(refTable))
}

func linkColumnFK(g *graph.Graph, tableName, colName, refTable string) {
	linkFK(g, graph.ColumnID(tableName, colName), graph.TableID(refTable))
}

func linkFK(g *graph.Graph, srcID, dstID string) {
	if g.Node(srcID) == nil || g.Node(dstID) == nil {
		return
	}
	edge := &graph.Edge{
		ID:    graph.EdgeID(graph.EdgeTypeReferencesFK, srcID, dstID),
		Type:  graph.EdgeTypeReferencesFK,
		SrcID: srcID,
		DstID: dstID,
	}
	_ = g.AddEdge(edge)
}
