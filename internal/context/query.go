package context

import (
	"fmt"
	"strings"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/summarizer"
)

// DefaultQuerySeeds is how many top lexical matches seed the query's
// subgraph expansion, per query-engine's natural-language subgraph
// requirement.
const DefaultQuerySeeds = 3

// Query answers a natural-language question by finding the most lexically
// relevant node(s) (summarizer.SearchNodes), expanding their subgraph, and
// rendering a token-budgeted, community-scoped context string — the
// query-engine evolution of GetContext (design.md Decision 5). hops <= 0
// uses DefaultHops; maxTokens <= 0 uses DefaultMaxTokens.
func Query(g *graph.Graph, s summarizer.SummaryReader, question string, hops, maxTokens int) (string, error) {
	if hops <= 0 {
		hops = DefaultHops
	}
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}

	results, err := summarizer.SearchNodes(g, s, question, DefaultQuerySeeds)
	if err != nil {
		return "", fmt.Errorf("context: searching for %q: %w", question, err)
	}
	if len(results) == 0 {
		return "", fmt.Errorf("context: no node found matching %q", question)
	}

	target := results[0].Node
	found := expandSubgraph(g, target, hops)
	// Merge in the remaining seeds so a question matching several relevant
	// nodes doesn't collapse to just the single best match — each seed is
	// treated as directly relevant (Hop 0) even though only the top seed
	// drives community-aware expansion.
	for _, r := range results[1:] {
		if _, ok := found[r.Node.ID]; !ok {
			found[r.Node.ID] = &discovered{Node: r.Node, Hop: 0}
		}
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Query: %s\n\n", question)
	rendered, err := renderContext(g, s, target, found, maxTokens)
	if err != nil {
		return "", fmt.Errorf("context: rendering query %q: %w", question, err)
	}
	out.WriteString(rendered)
	return out.String(), nil
}
