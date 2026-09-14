package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"kgraph/internal/export"
)

func newExportCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the full graph (nodes, edges, summaries, analytics) as graph.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, dbPath, err := resolvePaths()
			if err != nil {
				return fail(cmd, err)
			}
			g, s, closeFn, err := loadGraphAndStore(dbPath)
			if err != nil {
				return fail(cmd, err)
			}
			defer closeFn()

			if err := os.MkdirAll(output, 0o755); err != nil {
				return fail(cmd, fmt.Errorf("creating %s: %w", output, err))
			}

			data, err := export.ToJSON(g, s, repoPath)
			if err != nil {
				return fail(cmd, err)
			}
			jsonPath := filepath.Join(output, "graph.json")
			if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
				return fail(cmd, fmt.Errorf("writing %s: %w", jsonPath, err))
			}

			report, err := export.GenerateReport(g, s)
			if err != nil {
				return fail(cmd, err)
			}
			reportPath := filepath.Join(output, "GRAPH_REPORT.md")
			if err := os.WriteFile(reportPath, []byte(report), 0o644); err != nil {
				return fail(cmd, fmt.Errorf("writing %s: %w", reportPath, err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "exported %d nodes, %d edges to %s\n", g.NodeCount(), g.EdgeCount(), output)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "kgraph-out", "output directory for graph.json and GRAPH_REPORT.md")
	return cmd
}
