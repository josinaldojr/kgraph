package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// SummaryRecord is a stored summary at a given level ('node', 'file', or
// 'module').
type SummaryRecord struct {
	NodeID  string
	Level   string
	Hash    string
	Summary string
	Model   string
	Stale   bool
}

// SummaryHash returns the hash a stored summary was generated against, and
// whether one exists at all, for (nodeID, level).
func (s *Store) SummaryHash(nodeID, level string) (string, bool, error) {
	return summaryHash(s.db, nodeID, level)
}

// summaryHash holds SummaryHash's query logic, written against a plain
// *sql.DB so it's shared between Store (read-write) and ReadOnlyStore
// (read-only) instead of duplicated between them.
func summaryHash(db *sql.DB, nodeID, level string) (string, bool, error) {
	var hash string
	err := db.QueryRow(`SELECT hash FROM summaries WHERE node_id = ? AND level = ?`, nodeID, level).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("store: querying summary hash for %s/%s: %w", nodeID, level, err)
	}
	return hash, true, nil
}

// GetSummary returns the full stored summary record for (nodeID, level),
// including its stale flag, and whether one exists at all.
func (s *Store) GetSummary(nodeID, level string) (SummaryRecord, bool, error) {
	return getSummary(s.db, nodeID, level)
}

// getSummary holds GetSummary's query logic; see summaryHash's doc comment
// for why this is a free function shared with ReadOnlyStore.
func getSummary(db *sql.DB, nodeID, level string) (SummaryRecord, bool, error) {
	var r SummaryRecord
	var stale int
	err := db.QueryRow(`SELECT node_id, level, hash, summary, model, stale FROM summaries WHERE node_id = ? AND level = ?`,
		nodeID, level).Scan(&r.NodeID, &r.Level, &r.Hash, &r.Summary, &r.Model, &stale)
	if errors.Is(err, sql.ErrNoRows) {
		return SummaryRecord{}, false, nil
	}
	if err != nil {
		return SummaryRecord{}, false, fmt.Errorf("store: querying summary for %s/%s: %w", nodeID, level, err)
	}
	r.Stale = stale != 0
	return r, true, nil
}

// SummariesByLevel returns every stored summary at the given level, keyed
// by node ID, for bulk aggregation (file summaries reading their child
// node summaries, module summaries reading their child file summaries).
func (s *Store) SummariesByLevel(level string) (map[string]SummaryRecord, error) {
	return summariesByLevel(s.db, level)
}

// summariesByLevel holds SummariesByLevel's query logic; see summaryHash's
// doc comment for why this is a free function shared with ReadOnlyStore.
func summariesByLevel(db *sql.DB, level string) (map[string]SummaryRecord, error) {
	rows, err := db.Query(`SELECT node_id, level, hash, summary, model, stale FROM summaries WHERE level = ?`, level)
	if err != nil {
		return nil, fmt.Errorf("store: querying summaries at level %s: %w", level, err)
	}
	defer rows.Close()

	out := make(map[string]SummaryRecord)
	for rows.Next() {
		var r SummaryRecord
		var stale int
		if err := rows.Scan(&r.NodeID, &r.Level, &r.Hash, &r.Summary, &r.Model, &stale); err != nil {
			return nil, fmt.Errorf("store: scanning summary row: %w", err)
		}
		r.Stale = stale != 0
		out[r.NodeID] = r
	}
	return out, rows.Err()
}

// UpsertSummary stores a summary for (nodeID, level), keyed by hash.
func (s *Store) UpsertSummary(nodeID, level, hash, summary, model string) error {
	_, err := s.db.Exec(`
		INSERT INTO summaries (node_id, level, hash, summary, model, stale)
		VALUES (?, ?, ?, ?, ?, 0)
		ON CONFLICT(node_id, level) DO UPDATE SET
			hash=excluded.hash, summary=excluded.summary, model=excluded.model, stale=0`,
		nodeID, level, hash, summary, model)
	if err != nil {
		return fmt.Errorf("store: upserting summary for %s/%s: %w", nodeID, level, err)
	}
	return nil
}

// MarkSummaryStale flags an existing summary as stale without deleting it,
// so it's still available for diffing/fallback until a fresh one replaces
// it. Used by incremental update's direct-neighbor invalidation.
func (s *Store) MarkSummaryStale(nodeID, level string) error {
	_, err := s.db.Exec(`UPDATE summaries SET stale = 1 WHERE node_id = ? AND level = ?`, nodeID, level)
	if err != nil {
		return fmt.Errorf("store: marking summary stale for %s/%s: %w", nodeID, level, err)
	}
	return nil
}
