package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// ErrProjectLocked signals that the OS refused to delete a project's cache
// directory because its files are held open elsewhere — on Windows, a
// running `kgraph serve` keeps the database open and the filesystem won't
// release it. Callers (`kgraph prune`) report this as "skipped, in use" and
// continue with the remaining candidates instead of failing the whole run.
var ErrProjectLocked = errors.New("store: project files are locked by another process")

// RemoveProject deletes one project's entire cache directory — graph.db plus
// its -wal/-shm sidecars — identified by its cache key. Only reachable
// through explicit caller intent (the prune flow); discovery and serving
// never call this.
func RemoveProject(key string) error {
	root, err := CacheRoot()
	if err != nil {
		return err
	}
	return RemoveProjectFrom(root, key)
}

// RemoveProjectFrom is RemoveProject against an explicit cache root, so the
// prune flow can be tested without touching the real user cache.
//
// Safety: the key must be a plain relative directory name (no separators,
// no "..", no absolute paths) and the resolved directory must stay inside
// cacheRoot — nothing outside is ever touched. A nonexistent directory is
// not an error (deletion is idempotent).
func RemoveProjectFrom(cacheRoot, key string) error {
	if !filepath.IsLocal(key) {
		return fmt.Errorf("store: refusing to remove invalid project key %q", key)
	}
	dir := filepath.Join(cacheRoot, key)
	// Belt and suspenders: even with IsLocal, verify containment explicitly
	// before deleting anything.
	if rel, err := filepath.Rel(cacheRoot, dir); err != nil || rel != key {
		return fmt.Errorf("store: refusing to remove %q outside cache root %s", key, cacheRoot)
	}

	if err := os.RemoveAll(dir); err != nil {
		if isLockedError(err) {
			return fmt.Errorf("%w: %v", ErrProjectLocked, err)
		}
		return fmt.Errorf("store: removing project %s: %w", key, err)
	}
	return nil
}

// isLockedError reports whether err is the OS refusing deletion because a
// file is held open (Windows sharing violations) or a permissions error —
// the "a running serve has this database open" case from design.md's
// platform note.
func isLockedError(err error) bool {
	if errors.Is(err, fs.ErrPermission) {
		return true
	}
	if runtime.GOOS != "windows" {
		return false
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		return false
	}
	errno, ok := pathErr.Err.(syscall.Errno)
	if !ok {
		return false
	}
	// ERROR_ACCESS_DENIED (5), ERROR_SHARING_VIOLATION (32),
	// ERROR_LOCK_VIOLATION (33).
	return errno == 5 || errno == 32 || errno == 33
}
