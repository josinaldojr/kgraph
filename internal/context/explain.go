package context

import (
	"fmt"
	"sort"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/summarizer"
)

// Explain resolves nodeID (an exact ID, file path, or name — same
// resolution as GetContext's target) and returns its full-detail
// rendering: summary, direct relations (each tagged with its confidence),
// rationale, and analytics metadata (degree, community, god-node) — the
// query-engine's `explain` command. hops <= 1 renders only direct
// relations (the target's own section); hops > 1 additionally lists nodes
// reached by further expansion, same as GetContext's related-nodes list
// but unbounded by a token budget, since explain is meant to be
// exhaustive rather than budget-constrained.
func Explain(g *graph.Graph, s summarizer.SummaryReader, nodeID string, hops int) (string, error) {
	if hops <= 0 {
		hops = 1
	}
	target, err := resolveTarget(g, s, nodeID)
	if err != nil {
		return "", err
	}
	summaries, err := s.SummariesByLevel(summarizer.LevelNode)
	if err != nil {
		return "", fmt.Errorf("context: loading summaries: %w", err)
	}

	out := renderTargetSection(g, summaries, target)
	if hops <= 1 {
		return out, nil
	}

	found := expandSubgraph(g, target, hops)
	var others []*discovered
	for id, d := range found {
		if id != target.ID {
			others = append(others, d)
		}
	}
	sort.Slice(others, func(i, j int) bool {
		if others[i].Hop != others[j].Hop {
			return others[i].Hop < others[j].Hop
		}
		if others[i].Weight != others[j].Weight {
			return others[i].Weight > others[j].Weight
		}
		return others[i].Node.ID < others[j].Node.ID
	})
	if len(others) > 0 {
		out += "\nRelated nodes:\n"
		for _, d := range others {
			out += renderRelatedNode(summaries, d)
		}
	}
	return out, nil
}
