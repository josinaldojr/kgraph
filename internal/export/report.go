package export

import (
	"fmt"
	"sort"
	"strings"

	"kgraph/internal/graph"
	"kgraph/internal/store"
)

// maxReportGodNodes, maxReportSurprises bound how many entries GenerateReport
// lists in each section, so the report stays a skim-able summary rather
// than a full dump (that's what graph.json is for).
const (
	maxReportGodNodes  = 15
	maxReportSurprises = 10
)

// GenerateReport builds GRAPH_REPORT.md's content: god nodes ranked by
// degree, community descriptions, top cross-community edges ranked by
// surprise, and a handful of suggested questions derived from the graph's
// structure — per graph-analytics' "Graph report SHALL summarize
// analytics" requirement. s is accepted for symmetry with ToJSON and
// future use (e.g. attaching summaries to god nodes); it isn't read today.
func GenerateReport(g *graph.Graph, s *store.Store) (string, error) {
	_ = s
	var b strings.Builder

	b.WriteString("# Graph Report\n\n")
	fmt.Fprintf(&b, "%d nodes, %d edges.\n\n", g.NodeCount(), g.EdgeCount())

	writeGodNodes(&b, g)
	writeCommunities(&b, g)
	writeSurprisingConnections(&b, g)
	writeSuggestedQuestions(&b, g)

	return b.String(), nil
}

func writeGodNodes(b *strings.Builder, g *graph.Graph) {
	b.WriteString("## God Nodes\n\n")
	godNodes := g.GodNodes(maxReportGodNodes)
	if len(godNodes) == 0 {
		b.WriteString("None detected — run `kgraph analyze` first.\n\n")
		return
	}
	for _, n := range godNodes {
		fmt.Fprintf(b, "- `%s` (%s, degree %d) — %s\n", n.ID, n.Type, n.Degree(), n.File)
	}
	b.WriteString("\n")
}

func writeCommunities(b *strings.Builder, g *graph.Graph) {
	b.WriteString("## Communities\n\n")
	communities := g.Communities()
	if len(communities) == 0 {
		b.WriteString("None detected — run `kgraph analyze` first.\n\n")
		return
	}
	ids := make([]int, 0, len(communities))
	for id := range communities {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		members := communities[id]
		label := "Misc"
		if len(members) > 0 {
			label = members[0].CommunityLabel()
		}
		fmt.Fprintf(b, "- **%s** (community %d): %d nodes\n", label, id, len(members))
	}
	b.WriteString("\n")
}

type crossEdge struct {
	edge       *graph.Edge
	srcComm    int
	dstComm    int
	pairCount  int
	srcLabel   string
	dstLabel   string
	srcID      string
	dstID      string
	confidence string
}

// findCrossCommunityEdges returns every edge whose endpoints fall in
// different, assigned communities, each annotated with how many other
// edges connect that same community pair (pairCount) — the basis for the
// "surprise" ranking (a pair connected by only one or two edges is more
// surprising than one with dozens).
func findCrossCommunityEdges(g *graph.Graph) []crossEdge {
	pairCounts := make(map[[2]int]int)
	var candidates []crossEdge
	for _, e := range g.Edges() {
		src, dst := g.Node(e.SrcID), g.Node(e.DstID)
		if src == nil || dst == nil {
			continue
		}
		sc, dc := src.Community(), dst.Community()
		if sc == -1 || dc == -1 || sc == dc {
			continue
		}
		pair := [2]int{sc, dc}
		if sc > dc {
			pair = [2]int{dc, sc}
		}
		pairCounts[pair]++
		candidates = append(candidates, crossEdge{
			edge: e, srcComm: sc, dstComm: dc,
			srcLabel: src.CommunityLabel(), dstLabel: dst.CommunityLabel(),
			srcID: src.ID, dstID: dst.ID, confidence: e.Confidence,
		})
	}
	for i := range candidates {
		pair := [2]int{candidates[i].srcComm, candidates[i].dstComm}
		if pair[0] > pair[1] {
			pair[0], pair[1] = pair[1], pair[0]
		}
		candidates[i].pairCount = pairCounts[pair]
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].pairCount != candidates[j].pairCount {
			return candidates[i].pairCount < candidates[j].pairCount // rarer pair = more surprising
		}
		return candidates[i].edge.ID < candidates[j].edge.ID
	})
	return candidates
}

func writeSurprisingConnections(b *strings.Builder, g *graph.Graph) {
	b.WriteString("## Surprising Connections\n\n")
	crossing := findCrossCommunityEdges(g)
	if len(crossing) == 0 {
		b.WriteString("No cross-community edges found.\n\n")
		return
	}
	if len(crossing) > maxReportSurprises {
		crossing = crossing[:maxReportSurprises]
	}
	for _, c := range crossing {
		fmt.Fprintf(b, "- `%s` --%s(%s)--> `%s` (%s -> %s, %d edge(s) total between these communities)\n",
			c.srcID, c.edge.Type, c.confidence, c.dstID, c.srcLabel, c.dstLabel, c.pairCount)
	}
	b.WriteString("\n")
}

func writeSuggestedQuestions(b *strings.Builder, g *graph.Graph) {
	b.WriteString("## Suggested Questions\n\n")
	var questions []string

	godNodes := g.GodNodes(1)
	if len(godNodes) > 0 {
		questions = append(questions, fmt.Sprintf("What does `%s` do, and why is it so central (degree %d)?", godNodes[0].ID, godNodes[0].Degree()))
	}

	crossing := findCrossCommunityEdges(g)
	seenPairs := map[[2]string]bool{}
	for _, c := range crossing {
		key := [2]string{c.srcLabel, c.dstLabel}
		if c.srcLabel > c.dstLabel {
			key = [2]string{c.dstLabel, c.srcLabel}
		}
		if seenPairs[key] {
			continue
		}
		seenPairs[key] = true
		questions = append(questions, fmt.Sprintf("What connects %s to %s?", c.srcLabel, c.dstLabel))
		if len(questions) >= 5 {
			break
		}
	}

	communities := g.Communities()
	if len(communities) > 0 && len(questions) < 5 {
		ids := make([]int, 0, len(communities))
		for id := range communities {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			members := communities[id]
			if len(members) == 0 {
				continue
			}
			questions = append(questions, fmt.Sprintf("What is the responsibility of the %s subsystem?", members[0].CommunityLabel()))
			if len(questions) >= 5 {
				break
			}
		}
	}

	if len(questions) == 0 {
		b.WriteString("Run `kgraph analyze` first to unlock structure-derived questions.\n")
		return
	}
	for _, q := range questions {
		fmt.Fprintf(b, "- %s\n", q)
	}
}
