package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/josinaldojr/kgraph/internal/build"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Incrementally reprocess files changed since the last build/update",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, dbPath, err := resolvePaths()
			if err != nil {
				return fail(cmd, err)
			}

			res, err := build.Update(repoPath, dbPath)
			if err != nil {
				return fail(cmd, err)
			}
			for _, w := range res.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}

			if res.NoChange {
				fmt.Fprintf(cmd.OutOrStdout(), "up to date at %s (no changes since %s)\n", res.ToCommit, res.FromCommit)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s..%s: %d file(s) changed, %d node writes, %d edge writes, %d summaries marked stale\n",
				res.FromCommit, res.ToCommit, len(res.ChangedFiles), res.Stats.NodesWritten, res.Stats.EdgesWritten, res.StaleMarked)
			return nil
		},
	}
}
