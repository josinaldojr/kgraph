package store

import (
	"database/sql"
	"fmt"
	"time"
)

// LastCommit returns the last commit SHA processed for repoPath, and false
// if no build has been recorded for it yet.
func (s *Store) LastCommit(repoPath string) (string, bool, error) {
	return lastCommit(s.db, repoPath)
}

// lastCommit holds LastCommit's query logic; see LastBuildAt's doc comment
// for why this is a free function shared with ReadOnlyStore.
func lastCommit(db *sql.DB, repoPath string) (string, bool, error) {
	var commit string
	err := db.QueryRow(`SELECT last_commit FROM build_meta WHERE repo_path = ?`, repoPath).Scan(&commit)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("store: querying build_meta: %w", err)
	}
	return commit, commit != "", nil
}

// LastBuildAt returns the unix timestamp recorded by the most recent
// build/update for repoPath, and false if no build has been recorded yet.
// `kgraph serve`'s change-detection poller (internal/server) reads this on
// an interval: it's a single indexed-by-primary-key row, so polling it is
// far cheaper than reloading the graph on every tick just to check whether
// anything changed.
func (s *Store) LastBuildAt(repoPath string) (int64, bool, error) {
	return lastBuildAt(s.db, repoPath)
}

// lastBuildAt holds LastBuildAt's query logic, written against a plain
// *sql.DB so it's shared between Store (read-write) and ReadOnlyStore
// (read-only) instead of duplicated between them.
func lastBuildAt(db *sql.DB, repoPath string) (int64, bool, error) {
	var ts sql.NullInt64
	err := db.QueryRow(`SELECT last_build_at FROM build_meta WHERE repo_path = ?`, repoPath).Scan(&ts)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("store: querying build_meta: %w", err)
	}
	return ts.Int64, ts.Valid, nil
}

// SetLastCommit records the commit SHA a build/update finished processing,
// so the next `kgraph update` knows what to diff against.
func (s *Store) SetLastCommit(repoPath, commit string) error {
	_, err := s.db.Exec(`
		INSERT INTO build_meta (repo_path, last_commit, last_build_at)
		VALUES (?, ?, ?)
		ON CONFLICT(repo_path) DO UPDATE SET last_commit=excluded.last_commit, last_build_at=excluded.last_build_at`,
		repoPath, commit, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("store: recording build_meta: %w", err)
	}
	return nil
}
