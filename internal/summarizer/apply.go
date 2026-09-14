package summarizer

import (
	"fmt"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/store"
)

// AppliedSummary is one provider-written summary being stored back.
type AppliedSummary struct {
	ID      string `json:"id"`
	Hash    string `json:"hash"`
	Summary string `json:"summary"`
}

// ApplyResult reports what ApplySummaries actually did, so a mismatch is
// visible to the caller instead of silently dropped.
type ApplyResult struct {
	Applied []string
	Skipped []SkippedSummary
}

// SkippedSummary explains why one entry wasn't applied.
type SkippedSummary struct {
	ID     string
	Reason string
}

// ApplySummaries stores each entry into `summaries` at the given level,
// but only when entry.Hash still matches the target's *current* content
// hash: a node's current hash from the graph, or a file/module's current
// derived hash (recomputed the same way PendingSummaries computed it).
// Model identifies what produced the summaries (e.g. a provider/session
// id), stored verbatim per entry.
func ApplySummaries(g *graph.Graph, s *store.Store, level, model string, entries []AppliedSummary) (ApplyResult, error) {
	var currentHash func(id string) (string, bool, error)

	switch level {
	case LevelNode:
		currentHash = func(id string) (string, bool, error) {
			n := g.Node(id)
			if n == nil {
				return "", false, nil
			}
			return n.Hash, true, nil
		}
	case LevelFile, LevelModule:
		// Recompute the same derived hash pendingFiles/pendingModules used,
		// by re-running the pending scan and looking up the entry's current
		// hash from it (0 nodes changed since export is the common case, so
		// this is cheap and always consistent with what was exported).
		pending, err := PendingSummaries(g, s, level, 0)
		if err != nil {
			return ApplyResult{}, err
		}
		byID := make(map[string]string, len(pending))
		for _, p := range pending {
			byID[p.ID] = p.Hash
		}
		currentHash = func(id string) (string, bool, error) {
			h, ok := byID[id]
			return h, ok, nil
		}
	default:
		return ApplyResult{}, fmt.Errorf("summarizer: unknown level %q", level)
	}

	var res ApplyResult
	for _, e := range entries {
		want, ok, err := currentHash(e.ID)
		if err != nil {
			return res, err
		}
		if !ok {
			res.Skipped = append(res.Skipped, SkippedSummary{ID: e.ID, Reason: "unknown id"})
			continue
		}
		if want != e.Hash {
			res.Skipped = append(res.Skipped, SkippedSummary{ID: e.ID, Reason: "hash mismatch (code changed since export)"})
			continue
		}
		if err := s.UpsertSummary(e.ID, level, e.Hash, e.Summary, model); err != nil {
			return res, err
		}
		res.Applied = append(res.Applied, e.ID)
	}
	return res, nil
}
