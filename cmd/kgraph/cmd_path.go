package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"kgraph/internal/context"
)

func newPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path <src> <dst>",
		Short: "Find the shortest weighted path between two nodes",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, dbPath, err := resolvePaths()
			if err != nil {
				return fail(cmd, err)
			}
			g, _, closeFn, err := loadGraphAndStore(dbPath)
			if err != nil {
				return fail(cmd, err)
			}
			defer closeFn()

			hops, err := context.Path(g, args[0], args[1])
			if err != nil {
				return fail(cmd, err)
			}
			for i, h := range hops {
				if i == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n", h.Node.ID, h.Node.Type)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  --%s(%s)--> %s (%s)\n", h.ViaEdge, confidenceLabel(h.Confidence), h.Node.ID, h.Node.Type)
			}
			return nil
		},
	}
	return cmd
}

func confidenceLabel(c string) string {
	if c == "" {
		return "?"
	}
	return c
}
