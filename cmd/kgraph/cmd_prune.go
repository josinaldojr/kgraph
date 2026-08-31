package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/spf13/cobra"

	"kgraph/internal/store"
)

func newPruneCmd() *cobra.Command {
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete abandoned graph databases (repo missing or never built)",
		Long: "Delete abandoned graph databases: those whose recorded repository path no\n" +
			"longer exists on disk, plus databases with no build_meta row (created but\n" +
			"never built). A database whose repository exists is never a candidate,\n" +
			"regardless of how stale its build is. Deletion removes the project's whole\n" +
			"cache directory (graph.db plus WAL/SHM sidecars). Without --yes, candidates\n" +
			"are listed with size and reason and an explicit confirmation is required.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &pruneRunner{
				out:      cmd.OutOrStdout(),
				in:       cmd.InOrStdin(),
				discover: store.DiscoverProjects,
				remove:   store.RemoveProject,
				confirm:  confirmPrompt,
				dryRun:   dryRun,
				yes:      yes,
			}
			if err := r.run(); err != nil {
				return fail(cmd, err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list candidates without prompting or deleting anything")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation and delete all candidates")
	return cmd
}

// pruneCandidate is one discovered database `prune` offers for deletion,
// with the reason it qualifies.
type pruneCandidate struct {
	Info   store.ProjectInfo
	Reason string
}

// pruneCandidates selects the entries prune may offer: never-built databases
// (no build_meta row) and built databases whose recorded repo path no longer
// exists. Databases whose repo exists are never candidates regardless of
// staleness; unreadable databases are not candidates either — their repo
// path can't be read, so deleting them could destroy a live project's graph.
func pruneCandidates(projects []store.ProjectInfo) []pruneCandidate {
	var out []pruneCandidate
	for _, p := range projects {
		switch {
		case p.Status == store.StatusNeverBuilt:
			out = append(out, pruneCandidate{Info: p, Reason: "never built (no build recorded)"})
		case p.Status == store.StatusBuilt && store.ProjectFreshness(p) == store.FreshMissing:
			out = append(out, pruneCandidate{Info: p, Reason: "repository path no longer exists"})
		}
	}
	return out
}

// pruneRunner executes the prune flow against injectable collaborators so
// the command is unit-testable without touching the real user cache.
type pruneRunner struct {
	out      io.Writer
	in       io.Reader
	discover func() ([]store.ProjectInfo, error)
	remove   func(key string) error
	confirm  func(out io.Writer, in io.Reader, question string) (bool, error)
	dryRun   bool
	yes      bool
}

func (r *pruneRunner) run() error {
	projects, err := r.discover()
	if err != nil {
		return err
	}
	candidates := pruneCandidates(projects)
	if len(candidates) == 0 {
		fmt.Fprintln(r.out, "nothing to prune — every discovered database belongs to an existing repository and has a build recorded")
		return nil
	}

	fmt.Fprintf(r.out, "%d prune candidate(s):\n", len(candidates))
	for _, c := range candidates {
		fmt.Fprintf(r.out, "  %s  %s\n      %s · %s · %s\n",
			c.Info.Key, displayName(c.Info), humanSize(dirSize(c.Info.Dir)), c.Info.Dir, c.Reason)
	}

	if r.dryRun {
		fmt.Fprintln(r.out, "dry run — nothing deleted")
		return nil
	}
	if !r.yes {
		ok, err := r.confirm(r.out, r.in, fmt.Sprintf("Delete these %d project cache(s)? [y/N]: ", len(candidates)))
		if err != nil || !ok { // EOF (no user) or "no": never delete
			fmt.Fprintln(r.out, "aborted — nothing deleted")
			return nil
		}
	}

	var deleted, failed int
	for _, c := range candidates {
		if err := r.remove(c.Info.Key); err != nil {
			failed++
			if errors.Is(err, store.ErrProjectLocked) {
				fmt.Fprintf(r.out, "  skipped, in use: %s — %v\n", c.Info.Key, err)
				fmt.Fprintln(r.out, "      hint: stop any running `kgraph serve` that has this project open, then re-run prune")
			} else {
				fmt.Fprintf(r.out, "  skipped: %s — %v\n", c.Info.Key, err)
			}
			continue
		}
		deleted++
		fmt.Fprintf(r.out, "  deleted: %s (%s)\n", c.Info.Key, c.Info.Dir)
	}

	fmt.Fprintf(r.out, "%d deleted, %d skipped\n", deleted, failed)
	if deleted == 0 && failed > 0 {
		return fmt.Errorf("prune: every candidate failed to delete")
	}
	return nil
}

// dirSize sums the sizes of every file under dir (best-effort; unreadable
// files count as 0). Used to tell the user how much cache a candidate
// occupies before asking to delete it.
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// humanSize renders a byte count compactly for terminal output.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
