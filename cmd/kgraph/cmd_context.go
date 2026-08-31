package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"kgraph/internal/context"
)

func newContextCmd() *cobra.Command {
	var hops, maxTokens int
	cmd := &cobra.Command{
		Use:   "context <target>",
		Short: "Print a token-budgeted, summary-based context for a file/struct/function",
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

			out, err := context.GetContext(g, s, args[0], hops, maxTokens)
			if err != nil {
				return fail(cmd, err)
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().IntVar(&hops, "hops", context.DefaultHops, "how many edges to expand from the target")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", context.DefaultMaxTokens, "approximate token budget for the rendered context")
	return cmd
}
