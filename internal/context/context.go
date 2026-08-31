// Package context implements GetContext: resolving a target, expanding its
// subgraph, and rendering a token-budgeted, summary-based context string
// for an AI reviewer — never raw source code.
package context

import (
	"fmt"

	"kgraph/internal/graph"
	"kgraph/internal/store"
)

const (
	DefaultHops      = 2
	DefaultMaxTokens = 3000
)

// GetContext resolves target (an exact file/struct/function name, or a
// lexical search query as a fallback), expands its subgraph up to hops
// edges, and renders a token-budgeted context string built from node
// summaries and direct relations. hops <= 0 uses DefaultHops; maxTokens <=
// 0 uses DefaultMaxTokens.
func GetContext(g *graph.Graph, s *store.Store, target string, hops int, maxTokens int) (string, error) {
	if hops <= 0 {
		hops = DefaultHops
	}
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}

	node, err := resolveTarget(g, s, target)
	if err != nil {
		return "", err
	}

	found := expandSubgraph(g, node, hops)
	rendered, err := renderContext(g, s, node, found, maxTokens)
	if err != nil {
		return "", fmt.Errorf("context: rendering context for %q: %w", target, err)
	}
	return rendered, nil
}
