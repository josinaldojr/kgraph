// Package gitutil wraps the git CLI via os/exec for the small amount of
// git plumbing kgraph needs (current commit, changed files since a commit).
// go-git isn't used — a name-only diff and a rev-parse don't need it, per
// design.md's decision #9.
package gitutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// IsRepo reports whether dir is inside a git working tree.
func IsRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// CurrentCommit returns the current HEAD commit SHA for the repo at dir.
func CurrentCommit(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gitutil: git rev-parse HEAD in %s: %w", dir, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CurrentCommitTimeout is CurrentCommit bounded by a timeout, for
// best-effort lookups (e.g. `kgraph projects` freshness checks over many
// repos) where a wedged git process must not hang the whole operation.
func CurrentCommitTimeout(dir string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gitutil: git rev-parse HEAD in %s: %w", dir, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ChangedFiles returns the repo-relative paths of files that differ
// between fromCommit and HEAD.
func ChangedFiles(dir, fromCommit string) ([]string, error) {
	cmd := exec.Command("git", "-C", dir, "diff", "--name-only", fromCommit+"..HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gitutil: git diff --name-only %s..HEAD in %s: %w", fromCommit, dir, err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
