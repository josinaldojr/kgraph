package store

import (
	"database/sql"
	"fmt"
	"os"

	"kgraph/internal/graph"
)

// ReadOnlyStore is a restricted view over the graph database exposing only
// read operations. `kgraph serve` (internal/server) uses it instead of
// Store because it may run for a long time alongside a separate
// `kgraph build`/`kgraph update` process writing to the same database file
// (WAL mode supports one writer plus many concurrent readers, including
// across processes) — and it must never itself become a second writer.
// Unlike Store, ReadOnlyStore has no Save/Upsert/Delete/SetXxx methods at
// all, so a future addition to `serve` that tried to write would fail to
// compile instead of relying on code-review discipline to catch it.
type ReadOnlyStore struct {
	db *sql.DB
}

// OpenReadOnly opens the database at dbPath for reads only. It does not
// create the database or apply the schema: unlike Store.Open, a missing
// database here is an error the caller (`kgraph serve`) should surface as
// "run `kgraph build` first", not paper over by silently creating an empty
// one. The connection carries SQLite's `query_only` pragma, so any
// accidental write attempt fails at the database level, not just by
// convention.
func OpenReadOnly(dbPath string) (*ReadOnlyStore, error) {
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("store: no database at %s — run `kgraph build` first", dbPath)
		}
		return nil, fmt.Errorf("store: checking %s: %w", dbPath, err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_query_only=1")
	if err != nil {
		return nil, fmt.Errorf("store: opening %s read-only: %w", dbPath, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: opening %s read-only: %w", dbPath, err)
	}
	return &ReadOnlyStore{db: db}, nil
}

// Close closes the underlying database connection.
func (s *ReadOnlyStore) Close() error {
	return s.db.Close()
}

// LoadGraph reconstructs the in-memory Graph from the database.
func (s *ReadOnlyStore) LoadGraph() (*graph.Graph, error) {
	return loadGraph(s.db)
}

// GetSummary returns the full stored summary record for (nodeID, level).
func (s *ReadOnlyStore) GetSummary(nodeID, level string) (SummaryRecord, bool, error) {
	return getSummary(s.db, nodeID, level)
}

// SummariesByLevel returns every stored summary at the given level, keyed
// by node ID.
func (s *ReadOnlyStore) SummariesByLevel(level string) (map[string]SummaryRecord, error) {
	return summariesByLevel(s.db, level)
}

// SummaryHash returns the hash a stored summary was generated against, and
// whether one exists at all, for (nodeID, level).
func (s *ReadOnlyStore) SummaryHash(nodeID, level string) (string, bool, error) {
	return summaryHash(s.db, nodeID, level)
}

// LastBuildAt returns the unix timestamp recorded by the most recent
// build/update for repoPath, and false if no build has been recorded yet.
func (s *ReadOnlyStore) LastBuildAt(repoPath string) (int64, bool, error) {
	return lastBuildAt(s.db, repoPath)
}
