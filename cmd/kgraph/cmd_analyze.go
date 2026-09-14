package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"kgraph/internal/analytics"
)

func newAnalyzeCmd() *cobra.Command {
	var recluster bool
	var resolution float64
	var godNodes int

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Recompute graph analytics (degree, god nodes, communities) without re-parsing source",
		Args:  cobra.NoArgs,
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

			if recluster {
				analytics.DetectCommunities(g, resolution)
				analytics.LabelCommunities(g)
			} else if err := analytics.AnalyzeGraph(g, godNodes, resolution); err != nil {
				return fail(cmd, err)
			}

			if _, err := s.SaveGraph(g); err != nil {
				return fail(cmd, err)
			}
			if err := s.RefreshNodeProperties(g); err != nil {
				return fail(cmd, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "analyzed %d nodes across %d communities\n", g.NodeCount(), len(g.Communities()))
			return nil
		},
	}
	cmd.Flags().BoolVar(&recluster, "recluster", false, "only recompute communities (using --resolution), skipping degree/god-node recomputation")
	cmd.Flags().Float64Var(&resolution, "resolution", analytics.DefaultResolution, "community detection resolution — higher favors more, smaller communities")
	cmd.Flags().IntVar(&godNodes, "god-nodes", 0, "number of top-degree nodes to mark as god nodes (default 10)")
	return cmd
}
