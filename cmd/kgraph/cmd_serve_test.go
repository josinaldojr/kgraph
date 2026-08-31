package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"kgraph/internal/store"
)

func TestSelectServeMode(t *testing.T) {
	tests := []struct {
		name                 string
		pick, repoSet, dbSet bool
		want                 serveMode
	}{
		{name: "no project flags opens hub", want: serveHub},
		{name: "repo keeps legacy single mode", repoSet: true, want: serveSingle},
		{name: "explicit db keeps legacy single mode", dbSet: true, want: serveSingle},
		{name: "pick selects single mode", pick: true, want: serveSingle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectServeMode(tt.pick, tt.repoSet, tt.dbSet); got != tt.want {
				t.Fatalf("selectServeMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPickProjectSelectsBuiltGraph(t *testing.T) {
	origDiscover, origTerminal := discoverProjects, interactiveStdin
	t.Cleanup(func() {
		discoverProjects, interactiveStdin = origDiscover, origTerminal
	})
	discoverProjects = func() ([]store.ProjectInfo, error) {
		return []store.ProjectInfo{
			{Key: "never", Status: store.StatusNeverBuilt},
			{Key: "alpha", RepoPath: "/repos/alpha", DBPath: "/cache/alpha/graph.db", Status: store.StatusBuilt},
			{Key: "broken", Status: store.StatusUnreadable},
		}, nil
	}
	interactiveStdin = func() bool { return true }

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("1\n"))
	repoPath, dbPath, err := pickProject(cmd)
	if err != nil {
		t.Fatalf("pickProject() error = %v", err)
	}
	if repoPath != "/repos/alpha" || dbPath != "/cache/alpha/graph.db" {
		t.Fatalf("pickProject() = (%q, %q), want alpha paths", repoPath, dbPath)
	}
	if got := out.String(); !strings.Contains(got, "alpha") || strings.Contains(got, "never") || strings.Contains(got, "broken") {
		t.Errorf("picker must list only built projects, got:\n%s", got)
	}
}

func TestPickProjectRejectsNonInteractiveStdin(t *testing.T) {
	origDiscover, origTerminal := discoverProjects, interactiveStdin
	t.Cleanup(func() {
		discoverProjects, interactiveStdin = origDiscover, origTerminal
	})
	discoverProjects = func() ([]store.ProjectInfo, error) {
		return []store.ProjectInfo{{Key: "alpha", RepoPath: "/repos/alpha", DBPath: "/cache/alpha/graph.db", Status: store.StatusBuilt}}, nil
	}
	interactiveStdin = func() bool { return false }

	if _, _, err := pickProject(&cobra.Command{}); err == nil || !strings.Contains(err.Error(), "--repo") {
		t.Fatalf("pickProject() error = %v, want a non-TTY error pointing at --repo", err)
	}
}
