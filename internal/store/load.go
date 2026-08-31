package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"kgraph/internal/graph"
)

// LoadGraph reconstructs the in-memory Graph from the database: all nodes
// first, then all edges (AddEdge requires both endpoints to already exist).
func (s *Store) LoadGraph() (*graph.Graph, error) {
	return loadGraph(s.db)
}

// loadGraph holds LoadGraph's actual query logic, written against a plain
// *sql.DB so it's shared between Store (read-write) and ReadOnlyStore
// (read-only) instead of duplicated between them.
func loadGraph(db *sql.DB) (*graph.Graph, error) {
	g := graph.New()

	rows, err := db.Query(`SELECT id, type, file, line_start, line_end, signature, hash, properties FROM nodes`)
	if err != nil {
		return nil, fmt.Errorf("store: querying nodes: %w", err)
	}
	for rows.Next() {
		var n graph.Node
		var typ, propsJSON string
		if err := rows.Scan(&n.ID, &typ, &n.File, &n.LineStart, &n.LineEnd, &n.Signature, &n.Hash, &propsJSON); err != nil {
			rows.Close()
			return nil, fmt.Errorf("store: scanning node row: %w", err)
		}
		n.Type = graph.NodeType(typ)
		if propsJSON != "" {
			if err := json.Unmarshal([]byte(propsJSON), &n.Properties); err != nil {
				rows.Close()
				return nil, fmt.Errorf("store: unmarshaling properties for node %s: %w", n.ID, err)
			}
		}
		node := n
		g.AddNode(&node)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	erows, err := db.Query(`SELECT id, type, src_id, dst_id, properties FROM edges`)
	if err != nil {
		return nil, fmt.Errorf("store: querying edges: %w", err)
	}
	defer erows.Close()
	for erows.Next() {
		var e graph.Edge
		var typ, propsJSON string
		if err := erows.Scan(&e.ID, &typ, &e.SrcID, &e.DstID, &propsJSON); err != nil {
			return nil, fmt.Errorf("store: scanning edge row: %w", err)
		}
		e.Type = graph.EdgeType(typ)
		if propsJSON != "" {
			if err := json.Unmarshal([]byte(propsJSON), &e.Properties); err != nil {
				return nil, fmt.Errorf("store: unmarshaling properties for edge %s: %w", e.ID, err)
			}
		}
		edge := e
		if err := g.AddEdge(&edge); err != nil {
			return nil, fmt.Errorf("store: rebuilding graph: %w", err)
		}
	}
	if err := erows.Err(); err != nil {
		return nil, err
	}
	return g, nil
}
