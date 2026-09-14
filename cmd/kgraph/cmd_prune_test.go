package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kgraph/internal/store"
)

// gitRepoFixture creates a fresh git repository with one commit and returns
// its path and HEAD SHA (mirrors the helper in internal/store's tests).
func gitRepoFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	run("config", "user.email", "kgraph-test@example.com")
	run("config", "user.name", "kgraph test")
	run("commit", "-q", "--allow-empty", "-m", "initial")
	return dir, run("rev-parse", "HEAD")
}

func TestPruneCandidateSelectionRules(t *testing.T) {
	existingRepo, head := gitRepoFixture(t)

	projects := []store.ProjectInfo{
		// Orphaned: repo path gone → candidate.
		{Key: "aaaaaaaaaaaaaaaa", Status: store.StatusBuilt, RepoPath: filepath.Join(t.TempDir(), "gone"), LastCommit: "c1"},
		// Stale but existing: repo is there, build is behind HEAD → never a candidate.
		{Key: "bbbbbbbbbbbbbbbb", Status: store.StatusBuilt, RepoPath: existingRepo, LastCommit: strings.Repeat("0", 40)},
		// Up to date → never a candidate.
		{Key: "cccccccccccccccc", Status: store.StatusBuilt, RepoPath: existingRepo, LastCommit: head},
		// Never built → candidate.
		{Key: "dddddddddddddddd", Status: store.StatusNeverBuilt},
		// Unreadable → never a candidate (repo state unknown).
		{Key: "eeeeeeeeeeeeeeee", Status: store.StatusUnreadable},
	}

	candidates := pruneCandidates(projects)
	got := map[string]string{}
	for _, c := range candidates {
		got[c.Info.Key] = c.Reason
	}
	if len(candidates) != 2 {
		t.Fatalf("expected exactly 2 candidates, got %+v", candidates)
	}
	if got["aaaaaaaaaaaaaaaa"] == "" {
		t.Errorf("orphaned database must be a candidate, got %+v", got)
	}
	if got["dddddddddddddddd"] == "" {
		t.Errorf("never-built database must be a candidate, got %+v", got)
	}
	for _, key := range []string{"bbbbbbbbbbbbbbbb", "cccccccccccccccc", "eeeeeeeeeeeeeeee"} {
		if got[key] != "" {
			t.Errorf("key %s must not be a candidate, got %+v", key, got)
		}
	}
}

// newFixturePruneRunner wires a pruneRunner over a real temp cache root with
// N built fixture projects, optionally turning one into an orphan by
// pointing its repo path at a nonexistent directory.
func newFixturePruneRunner(t *testing.T, out *bytes.Buffer, in string) (*pruneRunner, string, []store.ProjectInfo) {
	t.Helper()
	root := t.TempDir()

	// Exists but is not a git repo → freshness unknown → never a candidate.
	liveRepo := t.TempDir()
	built := store.ProjectInfo{Key: "aaaaaaaaaaaaaaaa", Status: store.StatusBuilt, RepoPath: liveRepo, LastCommit: "c1", NodeCount: 1}
	orphan := store.ProjectInfo{Key: "bbbbbbbbbbbbbbbb", Status: store.StatusBuilt, RepoPath: filepath.Join(t.TempDir(), "gone"), LastCommit: "c2", NodeCount: 2}

	for _, p := range []store.ProjectInfo{built, orphan} {
		dbPath := filepath.Join(root, p.Key, "graph.db")
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		s, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if err := s.SetLastCommit(p.RepoPath, p.LastCommit); err != nil {
			t.Fatalf("SetLastCommit() error = %v", err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}

	runner := &pruneRunner{
		out:     out,
		in:      strings.NewReader(in),
		remove:  func(key string) error { return store.RemoveProjectFrom(root, key) },
		confirm: confirmPrompt,
	}
	runner.discover = func() ([]store.ProjectInfo, error) {
		return store.DiscoverProjectsWith(root)
	}
	return runner, root, []store.ProjectInfo{built, orphan}
}

func TestPruneDryRunDeletesNothing(t *testing.T) {
	var out bytes.Buffer
	runner, root, projects := newFixturePruneRunner(t, &out, "")
	runner.dryRun = true

	if err := runner.run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "bbbbbbbbbbbbbbbb") || !strings.Contains(rendered, "repository path no longer exists") {
		t.Errorf("dry run must list the orphaned candidate with its reason:\n%s", rendered)
	}
	if !strings.Contains(rendered, "dry run") {
		t.Errorf("dry run must say so:\n%s", rendered)
	}
	if strings.Contains(rendered, "[y/N]") {
		t.Errorf("dry run must not prompt:\n%s", rendered)
	}

	// Nothing deleted: both databases still discoverable.
	left, err := store.DiscoverProjectsWith(root)
	if err != nil {
		t.Fatalf("DiscoverProjectsWith() error = %v", err)
	}
	if len(left) != len(projects) {
		t.Errorf("dry run deleted something: %d projects remain, want %d", len(left), len(projects))
	}
}

func TestPruneConfirmedDeletesOnlyCandidates(t *testing.T) {
	var out bytes.Buffer
	runner, root, _ := newFixturePruneRunner(t, &out, "y\n")

	if err := runner.run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	left, err := store.DiscoverProjectsWith(root)
	if err != nil {
		t.Fatalf("DiscoverProjectsWith() error = %v", err)
	}
	if len(left) != 1 || left[0].Key != "aaaaaaaaaaaaaaaa" {
		t.Errorf("expected only the existing-repo project to survive, got %+v", left)
	}
	if !strings.Contains(out.String(), "deleted: bbbbbbbbbbbbbbbb") {
		t.Errorf("expected deletion report:\n%s", out.String())
	}
}

func TestPruneDeclinedDeletesNothing(t *testing.T) {
	var out bytes.Buffer
	runner, root, projects := newFixturePruneRunner(t, &out, "n\n")

	if err := runner.run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out.String(), "nothing deleted") {
		t.Errorf("expected an abort message:\n%s", out.String())
	}
	left, _ := store.DiscoverProjectsWith(root)
	if len(left) != len(projects) {
		t.Errorf("declining deleted something: %d projects remain", len(left))
	}
}

func TestPruneSkipsLockedCandidateAndContinues(t *testing.T) {
	var out bytes.Buffer
	runner, root, _ := newFixturePruneRunner(t, &out, "")
	runner.yes = true
	runner.remove = func(key string) error {
		if key == "bbbbbbbbbbbbbbbb" {
			return fmt.Errorf("%w: file held open by kgraph serve", store.ErrProjectLocked)
		}
		return store.RemoveProjectFrom(root, key)
	}

	// One locked candidate among... one candidate total means total failure.
	// Add a second, removable candidate to prove skip-and-continue.
	if err := runner.run(); err == nil {
		t.Fatalf("run() must fail when every candidate failed")
	}

	// Now with a deletable second candidate: the run continues past the lock.
	var out2 bytes.Buffer
	runner2, root2, _ := newFixturePruneRunner(t, &out2, "")
	runner2.yes = true
	neverBuiltDir := filepath.Join(root2, "dddddddddddddddd")
	if err := os.MkdirAll(neverBuiltDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	s, err := store.Open(filepath.Join(neverBuiltDir, "graph.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	runner2.remove = func(key string) error {
		if key == "bbbbbbbbbbbbbbbb" {
			return fmt.Errorf("%w: file held open by kgraph serve", store.ErrProjectLocked)
		}
		return store.RemoveProjectFrom(root2, key)
	}

	if err := runner2.run(); err != nil {
		t.Fatalf("run() error = %v (partial failure must still exit 0)", err)
	}
	rendered := out2.String()
	if !strings.Contains(rendered, "skipped, in use: bbbbbbbbbbbbbbbb") {
		t.Errorf("expected the locked candidate to be reported as skipped, in use:\n%s", rendered)
	}
	if !strings.Contains(rendered, "deleted: dddddddddddddddd") {
		t.Errorf("expected the remaining candidate to still be deleted:\n%s", rendered)
	}
	if !strings.Contains(rendered, "1 deleted, 1 skipped") {
		t.Errorf("expected a mixed summary:\n%s", rendered)
	}
}

func TestPruneNothingToPrune(t *testing.T) {
	var out bytes.Buffer
	runner := &pruneRunner{
		out: &out,
		discover: func() ([]store.ProjectInfo, error) {
			return []store.ProjectInfo{{Key: "aaaaaaaaaaaaaaaa", Status: store.StatusBuilt, RepoPath: t.TempDir(), LastCommit: "c1"}}, nil
		},
		remove: func(key string) error {
			t.Errorf("nothing should be removed, got remove(%q)", key)
			return nil
		},
		confirm: confirmPrompt,
	}
	if err := runner.run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out.String(), "nothing to prune") {
		t.Errorf("expected 'nothing to prune' message:\n%s", out.String())
	}
}
