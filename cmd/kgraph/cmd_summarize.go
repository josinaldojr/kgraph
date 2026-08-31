package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"kgraph/internal/summarizer"
)

func newSummarizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize",
		Short: "Provider-driven summarization: export pending nodes, apply written summaries",
	}
	cmd.AddCommand(newSummarizePendingCmd())
	cmd.AddCommand(newSummarizeApplyCmd())
	return cmd
}

func newSummarizePendingCmd() *cobra.Command {
	var level string
	var limit int
	cmd := &cobra.Command{
		Use:   "pending",
		Short: "Print nodes/files/modules lacking a current summary, as JSON, for a provider to summarize",
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

			pending, err := summarizer.PendingSummaries(g, s, level, limit)
			if err != nil {
				return fail(cmd, err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(pending)
		},
	}
	cmd.Flags().StringVar(&level, "level", summarizer.LevelNode, "node|file|module")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum entries to return (0 = unlimited)")
	return cmd
}

func newSummarizeApplyCmd() *cobra.Command {
	var level, model string
	cmd := &cobra.Command{
		Use:   "apply <file.json>",
		Short: "Apply provider-written summaries ({id, hash, summary}) from a JSON file",
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

			data, err := os.ReadFile(args[0])
			if err != nil {
				return fail(cmd, fmt.Errorf("reading %s: %w", args[0], err))
			}
			var entries []summarizer.AppliedSummary
			if err := json.Unmarshal(data, &entries); err != nil {
				return fail(cmd, fmt.Errorf("parsing %s: %w", args[0], err))
			}

			res, err := summarizer.ApplySummaries(g, s, level, model, entries)
			if err != nil {
				return fail(cmd, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "applied %d, skipped %d\n", len(res.Applied), len(res.Skipped))
			for _, sk := range res.Skipped {
				fmt.Fprintf(cmd.OutOrStdout(), "  skipped %s: %s\n", sk.ID, sk.Reason)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&level, "level", summarizer.LevelNode, "node|file|module")
	cmd.Flags().StringVar(&model, "model", "provider", "identifies what produced these summaries")
	return cmd
}
