// Package build wires internal/parser, internal/graph, and internal/store
// together into the kgraph build pipeline. This first pass covers
// extraction + persistence only (tasks.md group 5's checkpoint) —
// summarization and embeddings are layered on in a later pipeline stage,
// not this package's Run.
package build

import (
	"fmt"

	"kgraph/internal/analytics"
	"kgraph/internal/enrich"
	"kgraph/internal/gitutil"
	"kgraph/internal/graph"
	"kgraph/internal/parser"
	"kgraph/internal/store"
)

// Result reports what a Run produced, for CLI output (task 10.1) and for
// tests/benchmarks (task 5.2, 5.3, 9.4) to assert against.
type Result struct {
	Graph    *graph.Graph
	Stats    store.SaveStats
	Warnings []string
	Commit   string // HEAD commit recorded as the build baseline, "" if repoPath isn't a git repo
}

// Options configures the analyze stage of Run and Update.
type Options struct {
	// NoAnalyze skips the Analyze stage (degree/god-nodes/communities) —
	// per design.md's risk mitigation for large repos, this can be
	// re-run later via `kgraph analyze`. Enrich (rationale + confidence)
	// always runs, since edge-confidence requires every edge to carry a
	// Confidence value.
	NoAnalyze bool
	// GodNodeCount is the N passed to analytics.DetectGodNodes; <= 0
	// uses analytics.DefaultGodNodeCount.
	GodNodeCount int
}

// enrichAndAnalyze runs the Enrich stage (always) and, unless
// opts.NoAnalyze, the Analyze stage, over g in place. It's shared by Run
// and Update so both pipelines apply the same post-parse stages.
func enrichAndAnalyze(g *graph.Graph, opts Options) ([]string, error) {
	warnings, err := enrich.EnrichGraph(g)
	if err != nil {
		return warnings, fmt.Errorf("build: enriching graph: %w", err)
	}
	if !opts.NoAnalyze {
		if err := analytics.AnalyzeGraph(g, opts.GodNodeCount, analytics.DefaultResolution); err != nil {
			return warnings, fmt.Errorf("build: analyzing graph: %w", err)
		}
	}
	return warnings, nil
}

// Run extracts repoPath's full graph, runs the enrich/analyze stages over
// it (see Options), and persists it to the database at dbPath, recording
// the current commit as the baseline for future `kgraph update` calls
// (best-effort: repoPath not being a git repo is not an error here, since
// build/context/search don't require git).
func Run(repoPath, dbPath string, opts ...Options) (Result, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}

	g, warnings, err := parser.ExtractRepo(repoPath)
	if err != nil {
		return Result{}, fmt.Errorf("build: extracting %s: %w", repoPath, err)
	}

	enrichWarnings, err := enrichAndAnalyze(g, opt)
	warnings = append(warnings, enrichWarnings...)
	if err != nil {
		return Result{}, err
	}

	s, err := store.Open(dbPath)
	if err != nil {
		return Result{}, fmt.Errorf("build: opening store at %s: %w", dbPath, err)
	}
	defer s.Close()

	stats, err := s.SaveGraph(g)
	if err != nil {
		return Result{}, fmt.Errorf("build: saving graph: %w", err)
	}
	if err := s.RefreshNodeProperties(g); err != nil {
		return Result{}, fmt.Errorf("build: persisting analytics metadata: %w", err)
	}

	var commit string
	if gitutil.IsRepo(repoPath) {
		if c, err := gitutil.CurrentCommit(repoPath); err == nil {
			commit = c
		}
	}
	// Always record build_meta — including repoPath's last_build_at — even
	// when repoPath isn't a git repo (commit is then just ""). `kgraph
	// serve`'s change-detection poller (internal/server) and its
	// never-built check both key off last_build_at; gating this write on
	// gitutil.IsRepo, as an earlier version of this code did, silently
	// broke both for any non-git repository — build_meta would never gain
	// a row at all, even after a successful build.
	if err := s.SetLastCommit(repoPath, commit); err != nil {
		warnings = append(warnings, fmt.Sprintf("build: recording build metadata: %v", err))
	}

	return Result{Graph: g, Stats: stats, Warnings: warnings, Commit: commit}, nil
}
