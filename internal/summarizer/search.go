package summarizer

import (
	"regexp"
	"sort"
	"strings"

	"kgraph/internal/graph"
	"kgraph/internal/store"
)

// ScoredNode pairs a node with its lexical match score.
type ScoredNode struct {
	Node  *graph.Node
	Score int
}

var tokenRe = regexp.MustCompile(`[A-Za-z0-9_]+`)

func tokenize(s string) []string {
	matches := tokenRe.FindAllString(strings.ToLower(s), -1)
	return matches
}

// SummaryReader is the read subset SearchNodes needs — satisfied by both
// *store.Store and *store.ReadOnlyStore, so `kgraph serve` (which only
// ever holds a ReadOnlyStore, per its read-only requirement) can reuse the
// exact same lexical search the `kgraph search` CLI command and
// GetContext's fallback resolution use.
type SummaryReader interface {
	SummariesByLevel(level string) (map[string]store.SummaryRecord, error)
}

// SearchNodes ranks Struct/Interface/Function/Table/Endpoint/
// ExternalDependency nodes by term overlap between query and the node's
// summary — falling back to its signature and name when it has no summary
// yet — returning the topK highest-scoring nodes. No embeddings, no
// external API, per design.md Decision 7.
func SearchNodes(g *graph.Graph, s SummaryReader, query string, topK int) ([]ScoredNode, error) {
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 || topK <= 0 {
		return nil, nil
	}

	nodeSummaries, err := s.SummariesByLevel(LevelNode)
	if err != nil {
		return nil, err
	}

	var scored []ScoredNode
	for _, n := range g.Nodes() {
		if !summarizableTypes[n.Type] {
			continue
		}
		text := n.Signature
		if name, _ := n.Properties["name"].(string); name != "" {
			text += " " + name
		}
		if rec, ok := nodeSummaries[n.ID]; ok && rec.Summary != "" {
			text = rec.Summary + " " + text
		}

		score := termOverlapScore(queryTerms, text)
		if score > 0 {
			node := n
			scored = append(scored, ScoredNode{Node: node, Score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Node.ID < scored[j].Node.ID // stable tiebreak
	})
	if len(scored) > topK {
		scored = scored[:topK]
	}
	return scored, nil
}

// termOverlapScore counts, for each query term, how many times it appears
// in text's tokens.
func termOverlapScore(queryTerms []string, text string) int {
	textTerms := tokenize(text)
	counts := make(map[string]int, len(textTerms))
	for _, t := range textTerms {
		counts[t]++
	}
	score := 0
	for _, q := range queryTerms {
		score += counts[q]
	}
	return score
}
