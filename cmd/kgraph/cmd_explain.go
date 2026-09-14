package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"kgraph/internal/context"
)

func newExplainCmd() *cobra.Command {
	var hops int
	cmd := &cobra.Command{
		Use:   "explain <node>",
		Short: "Print full context for a node: summary, relations, rationale, and analytics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, dbPath, err := resolvePaths()
			if err != nil {
				return fail(cmd, err)
			}
			g, s, closeFn, err := loadGraphAndStore(dbPath)
			if err != nil {
				return fail(cmd, err)
			}
			defer closeFn()

			out, err := context.Explain(g, s, args[0], hops)
			if err != nil {
				return fail(cmd, err)
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().IntVar(&hops, "hops", 1, "how far beyond direct relations to include related nodes")
	return cmd
}
