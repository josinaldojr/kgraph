package context

import (
	"fmt"
	"sort"
	"strings"

	"kgraph/internal/graph"
	"kgraph/internal/store"
	"kgraph/internal/summarizer"
)

func summaryFor(summaries map[string]store.SummaryRecord, id string) string {
	if rec, ok := summaries[id]; ok && rec.Summary != "" {
		return rec.Summary
	}
	return "(no summary yet)"
}

// renderTargetSection renders the target node's own signature, summary,
// and full direct-relation lists (grouped by edge type, both directions) —
// this section is never truncated, per context-assembly's requirement to
// always keep the target's own summary and direct relations.
func renderTargetSection(g *graph.Graph, summaries map[string]store.SummaryRecord, target *graph.Node) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (%s)\n", target.ID, target.Type)
	if target.Signature != "" {
		fmt.Fprintf(&b, "Signature: %s\n", target.Signature)
	}
	fmt.Fprintf(&b, "Summary: %s\n", summaryFor(summaries, target.ID))

	type rel struct {
		label string
		ids   []string
	}
	byType := map[graph.EdgeType]*rel{}
	order := []graph.EdgeType{}
	addRel := func(edgeType graph.EdgeType, label, id string) {
		r, ok := byType[edgeType]
		if !ok {
			r = &rel{label: label}
			byType[edgeType] = r
			order = append(order, edgeType)
		}
		r.ids = append(r.ids, id)
	}
	for _, e := range g.OutEdges(target.ID) {
		addRel(e.Type, string(e.Type), e.DstID)
	}
	for _, e := range g.InEdges(target.ID) {
		addRel(e.Type, "called_by/referenced_by:"+string(e.Type), e.SrcID)
	}

	if len(order) > 0 {
		b.WriteString("Direct relations:\n")
		for _, et := range order {
			r := byType[et]
			sort.Strings(r.ids)
			fmt.Fprintf(&b, "  %s: %s\n", r.label, strings.Join(r.ids, ", "))
		}
	}
	return b.String()
}

// renderRelatedNode renders one non-target node's compact one-line(ish)
// entry: how it connects to the subgraph, plus its summary/signature —
// never raw source, per context-assembly's requirement.
func renderRelatedNode(summaries map[string]store.SummaryRecord, d *discovered) string {
	summary := summaryFor(summaries, d.Node.ID)
	if d.ViaOutgoing {
		// parent --edge--> this node
		return fmt.Sprintf("- [hop %d] %s --%s--> %s (%s): %s\n",
			d.Hop, d.ParentID, d.ViaEdge, d.Node.ID, d.Node.Type, summary)
	}
	// this node --edge--> parent
	return fmt.Sprintf("- [hop %d] %s --%s--> %s (%s): %s\n",
		d.Hop, d.Node.ID, d.ViaEdge, d.ParentID, d.Node.Type, summary)
}

// renderContext assembles the full context string: the target's mandatory
// section, then related nodes in priority order (nearest hop, then
// heaviest edge weight), truncated once the running token estimate would
// exceed maxTokens.
func renderContext(g *graph.Graph, s *store.Store, target *graph.Node, found map[string]*discovered, maxTokens int) (string, error) {
	summaries, err := s.SummariesByLevel(summarizer.LevelNode)
	if err != nil {
		return "", fmt.Errorf("context: loading summaries: %w", err)
	}

	var others []*discovered
	for id, d := range found {
		if id == target.ID {
			continue
		}
		others = append(others, d)
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

	var out strings.Builder
	targetSection := renderTargetSection(g, summaries, target)
	out.WriteString(targetSection)
	used := EstimateTokens(targetSection)

	if len(others) > 0 {
		header := "\nRelated nodes:\n"
		out.WriteString(header)
		used += EstimateTokens(header)
	}

	included, omitted := 0, 0
	for _, d := range others {
		block := renderRelatedNode(summaries, d)
		cost := EstimateTokens(block)
		if used+cost > maxTokens && included > 0 {
			omitted = len(others) - included
			break
		}
		out.WriteString(block)
		used += cost
		included++
	}
	if omitted > 0 {
		fmt.Fprintf(&out, "\n(%d further related node(s) omitted to stay within the %d-token budget)\n", omitted, maxTokens)
	}
	return out.String(), nil
}
