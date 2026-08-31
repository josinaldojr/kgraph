package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"kgraph/internal/build"
	"kgraph/internal/summarizer"
)

func newBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build [repo_path]",
		Short: "Extract a repository's code knowledge graph and persist it",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				repoFlag = args[0]
			}
			repoPath, dbPath, err := resolvePaths()
			if err != nil {
				return fail(cmd, err)
			}

			res, err := build.Run(repoPath, dbPath)
			if err != nil {
				return fail(cmd, err)
			}
			for _, w := range res.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}

			pendingCount, err := pendingNodeCount(repoPath, dbPath)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: counting pending summaries:", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "built %s: %d nodes, %d edges (%d node writes, %d edge writes)\n",
				repoPath, res.Graph.NodeCount(), res.Graph.EdgeCount(), res.Stats.NodesWritten, res.Stats.EdgesWritten)
			fmt.Fprintf(cmd.OutOrStdout(), "%d nodes still need a summary — run `kgraph summarize pending`\n", pendingCount)
			fmt.Fprintf(cmd.OutOrStdout(), "database: %s\n", filepath.Clean(dbPath))
			return nil
		},
	}
	return cmd
}

// pendingNodeCount is a lightweight helper shared by the build and
// summarize commands to report "N nodes still need a summary" without
// duplicating the store-open/graph-load dance.
func pendingNodeCount(repoPath, dbPath string) (int, error) {
	g, s, closeFn, err := loadGraphAndStore(dbPath)
	if err != nil {
		return 0, err
	}
	defer closeFn()

	pending, err := summarizer.PendingSummaries(g, s, summarizer.LevelNode, 0)
	if err != nil {
		return 0, err
	}
	return len(pending), nil
}
