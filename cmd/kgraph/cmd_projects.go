package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"kgraph/internal/store"
)

func newProjectsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "projects",
		Short: "List every built graph database in the kgraph cache",
		Long: "Discover every graph database under the per-user cache directory and print\n" +
			"one entry per database: display name, recorded repository path, last build\n" +
			"time, node/edge counts, and freshness relative to the repository's git HEAD.\n" +
			"Orphaned databases (repository path no longer exists) and never-built\n" +
			"databases are marked as prune candidates for `kgraph prune`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := store.DiscoverProjects()
			if err != nil {
				return fail(cmd, err)
			}
			if len(projects) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no built graphs — run `kgraph build` in a repository to create one")
				return nil
			}

			sort.Slice(projects, func(i, j int) bool {
				ni, nj := displayName(projects[i]), displayName(projects[j])
				if ni != nj {
					return ni < nj
				}
				return projects[i].RepoPath < projects[j].RepoPath
			})

			for _, p := range projects {
				fmt.Fprintln(cmd.OutOrStdout(), formatProject(p, store.ProjectFreshness(p)))
			}
			return nil
		},
	}
}

// displayName is the human-facing name for a discovered project: the
// repository basename when known, falling back to the opaque cache key for
// databases with no recorded repo path.
func displayName(p store.ProjectInfo) string {
	if p.RepoPath != "" {
		return filepath.Base(p.RepoPath)
	}
	return p.Key
}

// freshnessLabel renders a store.Freshness for terminal output.
func freshnessLabel(f store.Freshness) string {
	switch f {
	case store.FreshCurrent:
		return "up to date"
	case store.FreshStale:
		return "behind HEAD"
	case store.FreshMissing:
		return "repository missing"
	default:
		return "unknown"
	}
}

// isPruneCandidate reports whether `kgraph prune` would offer this entry:
// repo path missing (orphaned) or never built. Existing repos are never
// candidates regardless of staleness.
func isPruneCandidate(p store.ProjectInfo, fresh store.Freshness) bool {
	return fresh == store.FreshMissing || p.Status == store.StatusNeverBuilt
}

// formatProject renders one discovered project as terminal output (two
// lines), extracted so the output shape is unit-testable.
func formatProject(p store.ProjectInfo, fresh store.Freshness) string {
	head := displayName(p)
	if p.RepoPath != "" {
		head += " — " + p.RepoPath
	}
	if isPruneCandidate(p, fresh) {
		head += "  [prune candidate]"
	}

	var detail string
	switch p.Status {
	case store.StatusBuilt:
		detail = fmt.Sprintf("%d nodes, %d edges · built %s · %s",
			p.NodeCount, p.EdgeCount,
			time.Unix(p.LastBuildAt, 0).Format("2006-01-02 15:04"),
			freshnessLabel(fresh))
	case store.StatusNeverBuilt:
		detail = "never built (database created but no build recorded)"
	case store.StatusUnreadable:
		detail = "unreadable database — " + p.DBPath
	default:
		detail = string(p.Status)
	}
	return head + "\n    " + detail
}
