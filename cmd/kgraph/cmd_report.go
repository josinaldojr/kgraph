package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"kgraph/internal/export"
)

func newReportCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate GRAPH_REPORT.md: god nodes, communities, surprising connections, suggested questions",
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

			if err := os.MkdirAll(output, 0o755); err != nil {
				return fail(cmd, fmt.Errorf("creating %s: %w", output, err))
			}

			report, err := export.GenerateReport(g, s)
			if err != nil {
				return fail(cmd, err)
			}
			outPath := filepath.Join(output, "GRAPH_REPORT.md")
			if err := os.WriteFile(outPath, []byte(report), 0o644); err != nil {
				return fail(cmd, fmt.Errorf("writing %s: %w", outPath, err))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote report to %s\n", outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "kgraph-out", "output directory for GRAPH_REPORT.md")
	return cmd
}
