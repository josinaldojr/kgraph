package store

import (
	"os"
	"time"

	"github.com/josinaldojr/kgraph/internal/gitutil"
)

// Freshness classifies how current a built graph database is relative to its
// repository's current git HEAD, per kgraph-project-hub's design.md Decision 6.
type Freshness string

const (
	// FreshCurrent: the recorded last commit equals the repo's current HEAD.
	FreshCurrent Freshness = "current"
	// FreshStale: the repo's HEAD has moved past the recorded last commit.
	FreshStale Freshness = "stale"
	// FreshMissing: the recorded repository path no longer exists on disk.
	// Takes precedence over any git comparison — the project is a prune
	// candidate regardless of how recent the build is.
	FreshMissing Freshness = "missing"
	// FreshUnknown: git status couldn't be determined (path isn't a git
	// repo, git failed or timed out, no recorded repo path). Freshness is
	// best-effort and never fails discovery.
	FreshUnknown Freshness = "unknown"
)

// freshnessTimeout bounds each per-project `git rev-parse HEAD` call so one
// wedged git process can't hang `kgraph projects` or the hub's /api/projects.
const freshnessTimeout = 5 * time.Second

// ProjectFreshness determines p's freshness: missing when the recorded repo
// path doesn't exist (checked first, taking precedence over any git
// comparison), otherwise current/stale from comparing the recorded last
// commit against `git -C <repo> rev-parse HEAD`, degrading to unknown when
// git fails. Non-built entries have no meaningful freshness and report
// unknown. Never returns an error — freshness is best-effort by contract.
func ProjectFreshness(p ProjectInfo) Freshness {
	if p.Status != StatusBuilt || p.RepoPath == "" {
		return FreshUnknown
	}
	if _, err := os.Stat(p.RepoPath); err != nil {
		return FreshMissing
	}
	head, err := gitutil.CurrentCommitTimeout(p.RepoPath, freshnessTimeout)
	if err != nil {
		return FreshUnknown
	}
	if head == p.LastCommit {
		return FreshCurrent
	}
	return FreshStale
}
