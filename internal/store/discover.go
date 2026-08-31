package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ProjectStatus classifies a discovered graph database.
type ProjectStatus string

const (
	// StatusBuilt: the database opened fine and carries a build_meta row.
	StatusBuilt ProjectStatus = "built"
	// StatusNeverBuilt: the database opened fine but has no build_meta row —
	// it was created (schema applied) but never actually built.
	StatusNeverBuilt ProjectStatus = "never-built"
	// StatusUnreadable: the database could not be opened or read at all
	// (corrupt file, wrong format, ...). Discovery still reports it.
	StatusUnreadable ProjectStatus = "unreadable"
)

// ProjectInfo describes one discovered graph database under the kgraph cache
// directory. Key is the 16-hex cache directory name (the sha256 prefix of
// the repo path — see DefaultDBPath); it doubles as the project's identity
// in hub URLs and the runtime registry, per kgraph-project-hub's design.md
// Decision 2.
type ProjectInfo struct {
	Key         string        // cache directory name, e.g. "2b3915b6721d9932"
	Dir         string        // absolute path of the cache directory
	DBPath      string        // absolute path of graph.db inside Dir
	RepoPath    string        // recorded build_meta.repo_path ("" when unknown)
	LastCommit  string        // recorded build_meta.last_commit ("" when unknown)
	LastBuildAt int64         // recorded build_meta.last_build_at (unix seconds, 0 when unknown)
	NodeCount   int           // COUNT(*) over nodes (0 when unknown)
	EdgeCount   int           // COUNT(*) over edges (0 when unknown)
	Status      ProjectStatus // built | never-built | unreadable
}

// CacheRoot returns the directory holding every per-project cache directory:
// <UserCacheDir>/kgraph. Each project lives in <CacheRoot>/<key>/graph.db.
func CacheRoot() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("store: resolving user cache dir: %w", err)
	}
	return filepath.Join(cacheDir, "kgraph"), nil
}

// DiscoverProjects enumerates every graph database under the per-user cache
// directory. See DiscoverProjectsWith for the semantics.
func DiscoverProjects() ([]ProjectInfo, error) {
	root, err := CacheRoot()
	if err != nil {
		return nil, err
	}
	return DiscoverProjectsWith(root)
}

// DiscoverProjectsWith enumerates every <cacheRoot>/*/graph.db, opening each
// read-only to read its build_meta row plus cheap COUNT(*)s over nodes and
// edges, then closing it immediately.
//
// Discovery never writes: databases are opened with immutable=1 so no WAL/SHM
// sidecars are created and nothing on disk changes. The accepted trade-off —
// if a separate process has uncheckpointed WAL data at that exact moment,
// counts may lag one build behind; discovery is best-effort metadata, not
// the serving path (design.md's risk note on scanning every database).
//
// Unreadable/corrupt databases are returned as entries with StatusUnreadable
// rather than failing the whole scan; a missing cacheRoot yields an empty
// list. Entries are sorted by cache key for deterministic output.
func DiscoverProjectsWith(cacheRoot string) ([]ProjectInfo, error) {
	matches, err := filepath.Glob(filepath.Join(cacheRoot, "*", "graph.db"))
	if err != nil {
		return nil, fmt.Errorf("store: scanning %s: %w", cacheRoot, err)
	}
	sort.Strings(matches)

	projects := make([]ProjectInfo, 0, len(matches))
	for _, dbPath := range matches {
		projects = append(projects, inspectProjectDB(dbPath))
	}
	return projects, nil
}

// inspectProjectDB reads one database's metadata. It never returns an error:
// any failure becomes a StatusUnreadable entry, so one bad database can't
// break enumeration of the rest.
func inspectProjectDB(dbPath string) ProjectInfo {
	dir := filepath.Dir(dbPath)
	info := ProjectInfo{
		Key:    filepath.Base(dir),
		Dir:    dir,
		DBPath: dbPath,
		Status: StatusUnreadable,
	}

	// immutable=1 guarantees discovery performs no writes at all — no WAL/SHM
	// sidecar creation, no journal recovery. mode=ro opens read-only.
	dsn := "file:" + filepath.ToSlash(dbPath) + "?mode=ro&immutable=1&_pragma=query_only(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return info
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return info
	}

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM build_meta`).Scan(&count)
	if err != nil {
		return info // corrupt or not a kgraph database at all
	}
	if count == 0 {
		info.Status = StatusNeverBuilt
		return info
	}

	// One build_meta row per database is the norm (the DB path is derived
	// from the repo path); with several rows, prefer the most recently built
	// one for display metadata.
	err = db.QueryRow(`
		SELECT repo_path, COALESCE(last_commit, ''), COALESCE(last_build_at, 0)
		FROM build_meta ORDER BY COALESCE(last_build_at, 0) DESC LIMIT 1`).
		Scan(&info.RepoPath, &info.LastCommit, &info.LastBuildAt)
	if err != nil {
		return info
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&info.NodeCount); err != nil {
		return info
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM edges`).Scan(&info.EdgeCount); err != nil {
		return info
	}

	info.Status = StatusBuilt
	return info
}
