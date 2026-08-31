package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// setupMultiPackageGitRepo creates a throwaway git repo with numPkgs small
// Go packages (a handful of functions each), commits it, and returns its
// path. Enough packages exist so a full "./..." reprocess is measurably
// more expensive than a single-package scoped reprocess — the property
// task 9.4 checks — without needing a huge fixture.
func setupMultiPackageGitRepo(t *testing.T, numPkgs int) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=kgraph-test", "GIT_AUTHOR_EMAIL=kgraph-test@example.com",
			"GIT_COMMITTER_NAME=kgraph-test", "GIT_COMMITTER_EMAIL=kgraph-test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module updatefixture\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	for i := 0; i < numPkgs; i++ {
		pkgDir := filepath.Join(dir, fmt.Sprintf("pkg%d", i))
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", pkgDir, err)
		}
		writePackageFile(t, pkgDir, i, 0)
	}

	run("init", "-q")
	run("add", ".")
	run("commit", "-q", "-m", "initial")
	return dir
}

func writePackageFile(t *testing.T, pkgDir string, pkgIdx, variant int) {
	t.Helper()
	var src string
	src += fmt.Sprintf("package pkg%d\n\n", pkgIdx)
	for f := 0; f < 5; f++ {
		src += fmt.Sprintf("func F%d_%d() int {\n\treturn %d\n}\n\n", f, variant, f+variant)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "file.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("writing package file: %v", err)
	}
}

func TestUpdateOnlyReprocessesChangedPackage(t *testing.T) {
	const numPkgs = 8
	repo := setupMultiPackageGitRepo(t, numPkgs)
	dbPath := filepath.Join(t.TempDir(), "graph.db")

	if _, err := Run(repo, dbPath); err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}

	// Change only pkg3's file, commit, then update.
	changedPkgDir := filepath.Join(repo, "pkg3")
	writePackageFile(t, changedPkgDir, 3, 1) // variant 1 changes every function's body/hash
	gitCommit(t, repo, "change pkg3")

	res, err := Update(repo, dbPath)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if res.NoChange {
		t.Fatal("expected Update to detect the pkg3 change, got NoChange=true")
	}
	if len(res.ChangedFiles) != 1 || filepath.ToSlash(res.ChangedFiles[0]) != "pkg3/file.go" {
		t.Fatalf("expected exactly pkg3/file.go changed, got %+v", res.ChangedFiles)
	}
	// 5 functions in pkg3 rewritten -> 5 node writes; nothing from the
	// other 7 packages should have been touched.
	if res.Stats.NodesWritten == 0 {
		t.Errorf("expected some nodes written for the changed package, got 0")
	}
	t.Logf("changed files=%v nodesWritten=%d staleMarked=%d", res.ChangedFiles, res.Stats.NodesWritten, res.StaleMarked)
}

func TestUpdateFasterThanFullRebuild(t *testing.T) {
	const numPkgs = 40
	repo := setupMultiPackageGitRepo(t, numPkgs)
	dbPath := filepath.Join(t.TempDir(), "graph.db")

	if _, err := Run(repo, dbPath); err != nil {
		t.Fatalf("initial Run() error = %v", err)
	}

	writePackageFile(t, filepath.Join(repo, "pkg0"), 0, 1)
	gitCommit(t, repo, "change pkg0")

	updateStart := time.Now()
	if _, err := Update(repo, dbPath); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updateElapsed := time.Since(updateStart)

	fullDBPath := filepath.Join(t.TempDir(), "graph-full.db")
	buildStart := time.Now()
	if _, err := Run(repo, fullDBPath); err != nil {
		t.Fatalf("full Run() error = %v", err)
	}
	buildElapsed := time.Since(buildStart)

	t.Logf("update=%v full_build=%v (%d packages)", updateElapsed, buildElapsed, numPkgs)
	if updateElapsed >= buildElapsed {
		t.Errorf("expected incremental update (%v) to be faster than a full rebuild (%v) across %d packages",
			updateElapsed, buildElapsed, numPkgs)
	}
}

func gitCommit(t *testing.T, repo, msg string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=kgraph-test", "GIT_AUTHOR_EMAIL=kgraph-test@example.com",
			"GIT_COMMITTER_NAME=kgraph-test", "GIT_COMMITTER_EMAIL=kgraph-test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", ".")
	run("commit", "-q", "-m", msg)
}
