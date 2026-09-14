package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"kgraph/internal/context"
)

func newPromptCmd() *cobra.Command {
	var maxTokens int
	cmd := &cobra.Command{
		Use:   "prompt <node>",
		Short: "Render a node's context as an LLM-ready markdown prompt",
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

			out, err := context.Prompt(g, s, args[0], maxTokens)
			if err != nil {
				return fail(cmd, err)
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().IntVar(&maxTokens, "max-tokens", context.DefaultMaxTokens, "approximate token budget for the rendered prompt")
	return cmd
}
