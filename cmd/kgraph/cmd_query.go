package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"kgraph/internal/context"
)

func newQueryCmd() *cobra.Command {
	var hops, maxTokens int
	cmd := &cobra.Command{
		Use:   "query <question>",
		Short: "Answer a natural-language question with a relevant, token-budgeted subgraph",
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

			out, err := context.Query(g, s, args[0], hops, maxTokens)
			if err != nil {
				return fail(cmd, err)
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().IntVar(&hops, "hops", context.DefaultHops, "how many edges to expand from the best-matching node")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", context.DefaultMaxTokens, "approximate token budget for the rendered context")
	return cmd
}
