// Command kgraph builds and queries a compact code knowledge graph for a
// Go repository, meant as targeted, token-budgeted context for an AI code
// reviewer — see openspec/changes/kgraph-mvp for the full design.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/store"
)

var (
	repoFlag string
	dbFlag   string
)

func main() {
	root := &cobra.Command{
		Use:   "kgraph",
		Short: "Compact code knowledge graph for AI-assisted review",
	}
	root.PersistentFlags().StringVar(&repoFlag, "repo", ".", "path to the Go repository")
	root.PersistentFlags().StringVar(&dbFlag, "db", "", "path to the graph database (default: per-user cache dir keyed by --repo)")

	root.AddCommand(newBuildCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newContextCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newSummarizeCmd())
	root.AddCommand(newServeCmd())
	root.AddCommand(newProjectsCmd())
	root.AddCommand(newPruneCmd())
	root.AddCommand(newAnalyzeCmd())
	root.AddCommand(newQueryCmd())
	root.AddCommand(newPathCmd())
	root.AddCommand(newExplainCmd())
	root.AddCommand(newPromptCmd())
	root.AddCommand(newExportCmd())
	root.AddCommand(newReportCmd())
	root.AddCommand(newMCPCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// resolvePaths turns the --repo/--db flags into an absolute repo path and
// a concrete database path, applying store.DefaultDBPath when --db wasn't
// given.
func resolvePaths() (repoPath, dbPath string, err error) {
	repoPath, err = filepath.Abs(repoFlag)
	if err != nil {
		return "", "", fmt.Errorf("resolving --repo %q: %w", repoFlag, err)
	}
	if dbFlag != "" {
		return repoPath, dbFlag, nil
	}
	dbPath, err = store.DefaultDBPath(repoPath)
	if err != nil {
		return "", "", fmt.Errorf("resolving default database path: %w", err)
	}
	return repoPath, dbPath, nil
}

func fail(cmd *cobra.Command, err error) error {
	fmt.Fprintln(cmd.ErrOrStderr(), "error:", err)
	return err
}

// loadGraphAndStore opens the database at dbPath and loads its persisted
// graph, for commands (context, search, summarize) that query an
// already-built graph rather than re-extracting one.
func loadGraphAndStore(dbPath string) (*graph.Graph, *store.Store, func(), error) {
	s, err := store.Open(dbPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("opening database %s: %w", dbPath, err)
	}
	g, err := s.LoadGraph()
	if err != nil {
		s.Close()
		return nil, nil, nil, fmt.Errorf("loading graph from %s: %w", dbPath, err)
	}
	return g, s, func() { s.Close() }, nil
}
