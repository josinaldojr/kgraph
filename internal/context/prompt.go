package context

import (
	"fmt"
	"sort"
	"strings"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/summarizer"
)

// relatedNodeHops is how far past the target's direct relations `Prompt`
// looks for its "Related Nodes" section — one hop beyond Direct Relations,
// giving the LLM lightweight awareness of the target's neighborhood
// without a full second BFS hop's edge list (design.md Decision 4).
const relatedNodeHops = 2

// Prompt renders nodeID's full context as an LLM-ready prompt with the
// markdown sections query-engine's "Prompt formatted for LLM" scenario
// requires: "## Context: <id>", "### Direct Relations", "### Related
// Nodes", "### Rationale", "### Community" — a formatting layer over the
// same data Explain gathers, meant for pasting directly into a chat with
// an AI assistant (design.md Decision 7). maxTokens <= 0 uses
// DefaultMaxTokens.
func Prompt(g *graph.Graph, s summarizer.SummaryReader, nodeID string, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	target, err := resolveTarget(g, s, nodeID)
	if err != nil {
		return "", err
	}
	summaries, err := s.SummariesByLevel(summarizer.LevelNode)
	if err != nil {
		return "", fmt.Errorf("context: loading summaries: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Context: %s\n", target.ID)
	fmt.Fprintf(&b, "- Type: %s\n", target.Type)
	if target.Signature != "" {
		fmt.Fprintf(&b, "- Signature: `%s`\n", target.Signature)
	}
	fmt.Fprintf(&b, "- Summary: %s\n\n", summaryFor(summaries, target.ID))

	used := EstimateTokens(b.String())

	var directLines []string
	for _, e := range g.OutEdges(target.ID) {
		directLines = append(directLines, fmt.Sprintf("- %s --%s(%s)--> %s\n", target.ID, e.Type, confidenceOrUnknown(e.Confidence), e.DstID))
	}
	for _, e := range g.InEdges(target.ID) {
		directLines = append(directLines, fmt.Sprintf("- %s --%s(%s)--> %s\n", e.SrcID, e.Type, confidenceOrUnknown(e.Confidence), target.ID))
	}
	sort.Strings(directLines)
	if len(directLines) > 0 {
		used = appendSection(&b, used, maxTokens, "### Direct Relations\n", directLines)
	}

	var relatedLines []string
	for _, d := range sortedDiscovered(expandSubgraph(g, target, relatedNodeHops)) {
		if d.Node.ID == target.ID || d.Hop < relatedNodeHops {
			continue
		}
		relatedLines = append(relatedLines, fmt.Sprintf("- %s (%s): %s\n", d.Node.ID, d.Node.Type, summaryFor(summaries, d.Node.ID)))
	}
	if len(relatedLines) > 0 {
		used = appendSection(&b, used, maxTokens, "### Related Nodes\n", relatedLines)
	}

	if rationale := rationaleFor(g, target.ID); len(rationale) > 0 {
		b.WriteString("### Rationale\n")
		for _, r := range rationale {
			kind, _ := r.Properties["kind"].(string)
			text, _ := r.Properties["text"].(string)
			fmt.Fprintf(&b, "- **%s**: %s\n", kind, text)
		}
		b.WriteString("\n")
	}

	if label := target.CommunityLabel(); label != "" {
		b.WriteString("### Community\n")
		fmt.Fprintf(&b, "- %s (id %d)\n", label, target.Community())
		if target.IsGodNode() {
			fmt.Fprintf(&b, "- God node: yes (degree %d) — a heavily-connected hub; changes here have wide blast radius\n", target.Degree())
		}
	}

	return b.String(), nil
}

// appendSection writes header plus as many lines as fit within maxTokens
// (given used tokens already spent) to b, returning the updated used
// count. Truncation only ever drops trailing lines within a section — the
// header and any line that already fits are never removed.
func appendSection(b *strings.Builder, used, maxTokens int, header string, lines []string) int {
	var sb strings.Builder
	sb.WriteString(header)
	used += EstimateTokens(header)
	for _, line := range lines {
		cost := EstimateTokens(line)
		if used+cost > maxTokens {
			break
		}
		sb.WriteString(line)
		used += cost
	}
	sb.WriteString("\n")
	b.WriteString(sb.String())
	return used
}

// sortedDiscovered orders found the same way Explain/renderContext already
// do: nearest hop first, then heaviest edge weight, then ID for
// determinism.
func sortedDiscovered(found map[string]*discovered) []*discovered {
	out := make([]*discovered, 0, len(found))
	for _, d := range found {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hop != out[j].Hop {
			return out[i].Hop < out[j].Hop
		}
		if out[i].Weight != out[j].Weight {
			return out[i].Weight > out[j].Weight
		}
		return out[i].Node.ID < out[j].Node.ID
	})
	return out
}

func confidenceOrUnknown(c string) string {
	if c == "" {
		return "?"
	}
	return c
}
