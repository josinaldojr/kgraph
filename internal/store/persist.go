package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/josinaldojr/kgraph/internal/graph"
)

// SaveStats reports what a SaveGraph call actually wrote, distinguishing
// newly created/changed rows from ones left untouched because their
// content hash (nodes) or serialized properties (edges) were unchanged —
// this is what task 5.3 / the graph-storage spec's idempotency requirement
// verifies.
type SaveStats struct {
	NodesWritten   int
	NodesUnchanged int
	EdgesWritten   int
	EdgesUnchanged int
}

// SaveGraph persists every node and edge in g. A node is only written
// (INSERT OR REPLACE) when its content hash differs from what's already
// stored for that ID — an unchanged node is left untouched, including its
// updated_at timestamp, per graph-storage's "Idempotent persistence by
// content hash" requirement. Edges are written when their serialized
// properties differ from the existing row (or the row doesn't exist yet).
func (s *Store) SaveGraph(g *graph.Graph) (SaveStats, error) {
	var stats SaveStats

	tx, err := s.db.Begin()
	if err != nil {
		return stats, fmt.Errorf("store: begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op if committed

	for _, n := range g.Nodes() {
		var existingHash string
		err := tx.QueryRow(`SELECT hash FROM nodes WHERE id = ?`, n.ID).Scan(&existingHash)
		if err == nil && existingHash == n.Hash {
			stats.NodesUnchanged++
			continue
		}

		props, err := json.Marshal(n.Properties)
		if err != nil {
			return stats, fmt.Errorf("store: marshaling properties for node %s: %w", n.ID, err)
		}
		_, err = tx.Exec(`
			INSERT INTO nodes (id, type, file, line_start, line_end, signature, hash, properties, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				type=excluded.type, file=excluded.file, line_start=excluded.line_start,
				line_end=excluded.line_end, signature=excluded.signature, hash=excluded.hash,
				properties=excluded.properties, updated_at=excluded.updated_at`,
			n.ID, string(n.Type), n.File, n.LineStart, n.LineEnd, n.Signature, n.Hash, string(props), time.Now().Unix())
		if err != nil {
			return stats, fmt.Errorf("store: upserting node %s: %w", n.ID, err)
		}
		stats.NodesWritten++
	}

	for _, e := range g.Edges() {
		props, err := json.Marshal(e.Properties)
		if err != nil {
			return stats, fmt.Errorf("store: marshaling properties for edge %s: %w", e.ID, err)
		}

		confidence := e.Confidence
		if confidence == "" {
			confidence = graph.ConfidenceExtracted
		}

		var existingProps, existingConfidence string
		err = tx.QueryRow(`SELECT properties, confidence FROM edges WHERE id = ?`, e.ID).Scan(&existingProps, &existingConfidence)
		if err == nil && existingProps == string(props) && existingConfidence == confidence {
			stats.EdgesUnchanged++
			continue
		}

		_, err = tx.Exec(`
			INSERT INTO edges (id, type, src_id, dst_id, confidence, properties)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				type=excluded.type, src_id=excluded.src_id, dst_id=excluded.dst_id,
				confidence=excluded.confidence, properties=excluded.properties`,
			e.ID, string(e.Type), e.SrcID, e.DstID, confidence, string(props))
		if err != nil {
			return stats, fmt.Errorf("store: upserting edge %s: %w", e.ID, err)
		}
		stats.EdgesWritten++
	}

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("store: commit: %w", err)
	}
	return stats, nil
}

// RefreshNodeProperties unconditionally overwrites the properties column
// for every node in g that already exists in the database, independent of
// SaveGraph's content-hash gating. It exists for the analyze stage
// (internal/analytics): degree, god-node, and community metadata are
// derived from the whole graph's topology, so they can legitimately change
// even when a node's own content hash hasn't — graph-storage's "Idempotent
// persistence by content hash" guarantee is about *content* writes (the
// node's own code changing), not this graph-derived metadata, so this
// method deliberately bypasses that check rather than stretching SaveGraph
// to cover both meanings of "unchanged". A node not yet persisted (e.g. one
// SaveGraph is about to insert in the same build) is silently skipped here;
// SaveGraph already writes its properties as part of the insert.
func (s *Store) RefreshNodeProperties(g *graph.Graph) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op if committed

	for _, n := range g.Nodes() {
		props, err := json.Marshal(n.Properties)
		if err != nil {
			return fmt.Errorf("store: marshaling properties for node %s: %w", n.ID, err)
		}
		if _, err := tx.Exec(`UPDATE nodes SET properties = ? WHERE id = ?`, string(props), n.ID); err != nil {
			return fmt.Errorf("store: refreshing properties for node %s: %w", n.ID, err)
		}
	}
	return tx.Commit()
}

// DeleteNodesForFile removes every node (and its incident edges) whose
// file column matches path exactly — used by incremental update to clear
// out stale nodes for a changed/removed file before re-extracting it.
func (s *Store) DeleteNodesForFile(path string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	rows, err := tx.Query(`SELECT id FROM nodes WHERE file = ?`, path)
	if err != nil {
		return fmt.Errorf("store: querying nodes for file %s: %w", path, err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM edges WHERE src_id = ? OR dst_id = ?`, id, id); err != nil {
			return fmt.Errorf("store: deleting edges for node %s: %w", id, err)
		}
		if _, err := tx.Exec(`DELETE FROM summaries WHERE node_id = ?`, id); err != nil {
			return fmt.Errorf("store: deleting summaries for node %s: %w", id, err)
		}
		if _, err := tx.Exec(`DELETE FROM nodes WHERE id = ?`, id); err != nil {
			return fmt.Errorf("store: deleting node %s: %w", id, err)
		}
	}
	return tx.Commit()
}
