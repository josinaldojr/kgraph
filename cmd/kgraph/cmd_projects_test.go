package main

import (
	"strings"
	"testing"

	"kgraph/internal/store"
)

func TestFormatProjectBuiltUpToDate(t *testing.T) {
	p := store.ProjectInfo{
		Key: "aaaaaaaaaaaaaaaa", RepoPath: `C:\repos\alpha`,
		Status: store.StatusBuilt, NodeCount: 123, EdgeCount: 45, LastBuildAt: 1788000000,
	}
	out := formatProject(p, store.FreshCurrent)

	for _, want := range []string{
		"alpha",               // display name = repo basename
		`C:\repos\alpha`,      // full path
		"123 nodes, 45 edges", // counts
		"up to date",          // freshness
		"2026-",               // last build time rendered
	} {
		if !strings.Contains(out, want) {
			t.Errorf("built entry missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "prune candidate") {
		t.Errorf("an up-to-date built project must not be a prune candidate:\n%s", out)
	}
}

func TestFormatProjectOrphanedIsPruneCandidate(t *testing.T) {
	p := store.ProjectInfo{
		Key: "bbbbbbbbbbbbbbbb", RepoPath: `C:\gone\oldrepo`,
		Status: store.StatusBuilt, NodeCount: 10, EdgeCount: 2, LastBuildAt: 1788000000,
	}
	out := formatProject(p, store.FreshMissing)

	for _, want := range []string{"oldrepo", `C:\gone\oldrepo`, "repository missing", "[prune candidate]"} {
		if !strings.Contains(out, want) {
			t.Errorf("orphaned entry missing %q:\n%s", want, out)
		}
	}
}

func TestFormatProjectNeverBuilt(t *testing.T) {
	p := store.ProjectInfo{Key: "dddddddddddddddd", Status: store.StatusNeverBuilt}
	out := formatProject(p, store.FreshUnknown)

	for _, want := range []string{"dddddddddddddddd", "never built", "[prune candidate]"} {
		if !strings.Contains(out, want) {
			t.Errorf("never-built entry missing %q:\n%s", want, out)
		}
	}
}

func TestFormatProjectUnreadable(t *testing.T) {
	p := store.ProjectInfo{
		Key: "cccccccccccccccc", Status: store.StatusUnreadable,
		DBPath: `C:\cache\kgraph\cccccccccccccccc\graph.db`,
	}
	out := formatProject(p, store.FreshUnknown)

	for _, want := range []string{"cccccccccccccccc", "unreadable", `graph.db`} {
		if !strings.Contains(out, want) {
			t.Errorf("unreadable entry missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "prune candidate") {
		t.Errorf("unreadable databases are not prune candidates (their repo may still exist):\n%s", out)
	}
}

func TestFormatProjectStaleButExistingNotCandidate(t *testing.T) {
	p := store.ProjectInfo{
		Key: "eeeeeeeeeeeeeeee", RepoPath: `C:\repos\behind`,
		Status: store.StatusBuilt, NodeCount: 1, EdgeCount: 0, LastBuildAt: 1788000000,
	}
	out := formatProject(p, store.FreshStale)

	if !strings.Contains(out, "behind HEAD") {
		t.Errorf("expected 'behind HEAD' freshness, got:\n%s", out)
	}
	if strings.Contains(out, "prune candidate") {
		t.Errorf("a stale build of an existing repo must never be a prune candidate:\n%s", out)
	}
}
