package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// DefaultDBPath resolves the default SQLite database location for
// repoPath: an external per-user cache directory keyed by the repo's
// absolute path, so kgraph never writes into the analyzed repository
// itself (mirroring knowledge-cli's MCP-memory mode, which deliberately
// writes nothing into observed repos — see design.md's "Open Questions").
// Callers that want the database inside the repo instead can pass an
// explicit path via the CLI's --db flag rather than using this default.
func DefaultDBPath(repoPath string) (string, error) {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return "", err
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(abs))
	key := hex.EncodeToString(sum[:])[:16]
	return filepath.Join(cacheDir, "kgraph", key, "graph.db"), nil
}
