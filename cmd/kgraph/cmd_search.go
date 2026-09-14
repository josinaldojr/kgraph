package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/josinaldojr/kgraph/internal/summarizer"
)

func newSearchCmd() *cobra.Command {
	var topK int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Lexically search node summaries/names",
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

			results, err := summarizer.SearchNodes(g, s, args[0], topK)
			if err != nil {
				return fail(cmd, err)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no matches")
				return nil
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-6d %-10s %-30s %s\n", r.Score, r.Node.Type, r.Node.ID, r.Node.File)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&topK, "top", 10, "maximum number of results")
	return cmd
}
