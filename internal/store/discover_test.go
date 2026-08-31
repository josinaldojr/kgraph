package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kgraph/internal/graph"
)

// buildFixtureProject creates <cacheRoot>/<key>/graph.db as a fully built
// database (graph + build_meta row) for repoPath at lastCommit, using the
// real write path (Store) so fixtures match what `kgraph build` produces.
func buildFixtureProject(t *testing.T, cacheRoot, key, repoPath, lastCommit string, nodeCount int) string {
	t.Helper()
	dbPath := filepath.Join(cacheRoot, key, "graph.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open(%s) error = %v", dbPath, err)
	}
	defer s.Close()

	g := graph.New()
	g.AddNode(&graph.Node{ID: "pkg:demo", Type: graph.NodeTypePackage, Hash: "h1"})
	for i := 1; i < nodeCount; i++ {
		g.AddNode(&graph.Node{ID: "demo.Fn" + string(rune('0'+i)) + "()", Type: graph.NodeTypeFunction, Hash: "h"})
	}
	if _, err := s.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph() error = %v", err)
	}
	if err := s.SetLastCommit(repoPath, lastCommit); err != nil {
		t.Fatalf("SetLastCommit() error = %v", err)
	}
	return dbPath
}

func TestDiscoverProjectsEnumeratesBuiltDatabases(t *testing.T) {
	root := t.TempDir()
	buildFixtureProject(t, root, "aaaaaaaaaaaaaaaa", "/repo/alpha", "c1aaa", 3)
	buildFixtureProject(t, root, "bbbbbbbbbbbbbbbb", "/repo/beta", "c2bbb", 5)

	projects, err := DiscoverProjectsWith(root)
	if err != nil {
		t.Fatalf("DiscoverProjectsWith() error = %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 discovered projects, got %d: %+v", len(projects), projects)
	}

	byKey := map[string]ProjectInfo{}
	for _, p := range projects {
		byKey[p.Key] = p
	}
	a, ok := byKey["aaaaaaaaaaaaaaaa"]
	if !ok {
		t.Fatalf("expected project aaaaaaaaaaaaaaaa, got %+v", projects)
	}
	if a.Status != StatusBuilt || a.RepoPath != "/repo/alpha" || a.LastCommit != "c1aaa" {
		t.Errorf("bad metadata for alpha: %+v", a)
	}
	if a.LastBuildAt == 0 {
		t.Errorf("expected a recorded last_build_at for alpha, got %+v", a)
	}
	if a.NodeCount != 3 || a.EdgeCount != 0 {
		t.Errorf("expected 3 nodes / 0 edges for alpha, got %d/%d", a.NodeCount, a.EdgeCount)
	}
	b := byKey["bbbbbbbbbbbbbbbb"]
	if b.RepoPath != "/repo/beta" || b.NodeCount != 5 {
		t.Errorf("bad metadata for beta: %+v", b)
	}
}

func TestDiscoverProjectsToleratesCorruptDatabase(t *testing.T) {
	root := t.TempDir()
	buildFixtureProject(t, root, "aaaaaaaaaaaaaaaa", "/repo/alpha", "c1", 2)

	// A corrupt database: garbage bytes instead of a SQLite file.
	corrupt := filepath.Join(root, "cccccccccccccccc", "graph.db")
	if err := os.MkdirAll(filepath.Dir(corrupt), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(corrupt, []byte("this is not a sqlite database"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	projects, err := DiscoverProjectsWith(root)
	if err != nil {
		t.Fatalf("DiscoverProjectsWith() error = %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected the corrupt DB to be reported alongside the good one, got %+v", projects)
	}
	byKey := map[string]ProjectInfo{}
	for _, p := range projects {
		byKey[p.Key] = p
	}
	if byKey["cccccccccccccccc"].Status != StatusUnreadable {
		t.Errorf("expected corrupt DB reported as unreadable, got %+v", byKey["cccccccccccccccc"])
	}
	if byKey["aaaaaaaaaaaaaaaa"].Status != StatusBuilt {
		t.Errorf("expected the good DB to survive the corrupt one, got %+v", byKey["aaaaaaaaaaaaaaaa"])
	}
}

func TestDiscoverProjectsReportsNeverBuiltDatabase(t *testing.T) {
	root := t.TempDir()

	// Schema applied but no build_meta row: what `store.Open` followed by an
	// interrupted/abandoned build leaves behind.
	dbPath := filepath.Join(root, "dddddddddddddddd", "graph.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	projects, err := DiscoverProjectsWith(root)
	if err != nil {
		t.Fatalf("DiscoverProjectsWith() error = %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 discovered project, got %+v", projects)
	}
	if projects[0].Status != StatusNeverBuilt {
		t.Errorf("expected never-built status for a schema-only DB, got %+v", projects[0])
	}
}

func TestDiscoverProjectsIsReadOnly(t *testing.T) {
	root := t.TempDir()
	dbPath := buildFixtureProject(t, root, "aaaaaaaaaaaaaaaa", "/repo/alpha", "c1", 2)

	before, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	projects, err := DiscoverProjectsWith(root)
	if err != nil {
		t.Fatalf("DiscoverProjectsWith() error = %v", err)
	}
	if len(projects) != 1 || projects[0].Status != StatusBuilt {
		t.Fatalf("unexpected discovery result: %+v", projects)
	}

	after, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Stat() after discovery: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Errorf("discovery modified the database: before=%+v after=%+v", before, after)
	}

	entries, err := os.ReadDir(filepath.Join(root, "aaaaaaaaaaaaaaaa"))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, e := range entries {
		if e.Name() != "graph.db" {
			t.Errorf("discovery created a file: %s", e.Name())
		}
	}
}

func TestDiscoverProjectsEmptyRoot(t *testing.T) {
	projects, err := DiscoverProjectsWith(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("DiscoverProjectsWith() on a missing root error = %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected no projects for a missing cache root, got %+v", projects)
	}
}

// ---- Freshness ----

// initGitRepo creates a fresh git repository with one commit and returns its
// path and HEAD SHA.
func initGitRepo(t *testing.T) (string, string) {
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

func TestProjectFreshnessMatchesHead(t *testing.T) {
	repo, head := initGitRepo(t)
	p := ProjectInfo{Status: StatusBuilt, RepoPath: repo, LastCommit: head}
	if got := ProjectFreshness(p); got != FreshCurrent {
		t.Errorf("expected current when last_commit == HEAD, got %q", got)
	}
}

func TestProjectFreshnessBehindHead(t *testing.T) {
	repo, _ := initGitRepo(t)
	p := ProjectInfo{Status: StatusBuilt, RepoPath: repo, LastCommit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}
	if got := ProjectFreshness(p); got != FreshStale {
		t.Errorf("expected stale when HEAD moved on, got %q", got)
	}
}

func TestProjectFreshnessMissingPathTakesPrecedence(t *testing.T) {
	p := ProjectInfo{
		Status:     StatusBuilt,
		RepoPath:   filepath.Join(t.TempDir(), "no-longer-here"),
		LastCommit: "anything",
	}
	if got := ProjectFreshness(p); got != FreshMissing {
		t.Errorf("expected missing for a nonexistent repo path, got %q", got)
	}
}

func TestProjectFreshnessNonGitPathIsUnknown(t *testing.T) {
	p := ProjectInfo{Status: StatusBuilt, RepoPath: t.TempDir(), LastCommit: "c1"}
	if got := ProjectFreshness(p); got != FreshUnknown {
		t.Errorf("expected unknown for a path that isn't a git repo, got %q", got)
	}
}

func TestProjectFreshnessNonBuiltIsUnknown(t *testing.T) {
	for _, status := range []ProjectStatus{StatusNeverBuilt, StatusUnreadable} {
		p := ProjectInfo{Status: status, RepoPath: "/somewhere"}
		if got := ProjectFreshness(p); got != FreshUnknown {
			t.Errorf("expected unknown for status %s, got %q", status, got)
		}
	}
}
