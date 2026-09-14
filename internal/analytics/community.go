package analytics

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/josinaldojr/kgraph/internal/graph"
)

// DefaultResolution is the resolution `kgraph build`/`kgraph analyze` use
// when none is given. Values above 1.0 favor more, smaller communities;
// values below 1.0 favor fewer, larger ones (see mergeSmallCommunities).
const DefaultResolution = 1.0

// maxLabelPropagationIterations bounds the label-propagation loop so a
// pathological graph can't spin forever; well-behaved code graphs converge
// in a handful of passes.
const maxLabelPropagationIterations = 20

// DetectCommunities partitions g into communities using label propagation
// — a seed-free heuristic clustering algorithm well suited to the sparse,
// naturally-clustered graphs code repositories produce (see design.md
// Decision 4): every node starts in its own community, then repeatedly
// adopts the community held by the plurality of its neighbors (by edge
// count, both directions) until the assignment stabilizes. resolution
// then merges communities smaller than a size threshold derived from it
// into their most-connected neighboring community — lower resolution
// merges more aggressively (fewer, larger communities), higher resolution
// merges less (more, smaller communities). Sets Properties["community"]
// on every node to a small integer, densest community first.
func DetectCommunities(g *graph.Graph, resolution float64) {
	nodes := g.Nodes()
	if len(nodes) == 0 {
		return
	}
	if resolution <= 0 {
		resolution = DefaultResolution
	}

	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	sort.Strings(ids)

	neighbors := undirectedNeighbors(g)

	comm := make(map[string]int, len(ids))
	for i, id := range ids {
		comm[id] = i
	}

	for iter := 0; iter < maxLabelPropagationIterations; iter++ {
		changed := false
		for _, id := range ids {
			best, bestCount := comm[id], -1
			counts := make(map[int]int)
			for neighbor, weight := range neighbors[id] {
				c := comm[neighbor]
				counts[c] += weight
			}
			// Deterministic tie-break: prefer the current community to
			// reduce churn, then the smallest community ID.
			var candidates []int
			for c := range counts {
				candidates = append(candidates, c)
			}
			sort.Ints(candidates)
			for _, c := range candidates {
				if counts[c] > bestCount || (counts[c] == bestCount && c == comm[id]) {
					best, bestCount = c, counts[c]
				}
			}
			if best != comm[id] {
				comm[id] = best
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	mergeSmallCommunities(ids, comm, neighbors, resolution)
	renumberCommunities(ids, comm)

	for _, id := range ids {
		n := g.Node(id)
		if n.Properties == nil {
			n.Properties = make(map[string]any)
		}
		n.Properties[graph.PropertyCommunity] = comm[id]
	}
}

// undirectedNeighbors builds, for every node, a map of neighbor ID to
// edge count (treating the graph as undirected and unweighted-per-edge —
// label propagation only cares about connection strength, not edge
// semantics).
func undirectedNeighbors(g *graph.Graph) map[string]map[string]int {
	out := make(map[string]map[string]int)
	add := func(a, b string) {
		if out[a] == nil {
			out[a] = make(map[string]int)
		}
		out[a][b]++
	}
	for _, e := range g.Edges() {
		if e.SrcID == e.DstID {
			continue
		}
		add(e.SrcID, e.DstID)
		add(e.DstID, e.SrcID)
	}
	return out
}

// mergeSmallCommunities folds any community smaller than a
// resolution-derived threshold into whichever neighboring community it's
// most strongly connected to, so a handful of stray singletons from label
// propagation don't each become their own community.
func mergeSmallCommunities(ids []string, comm map[string]int, neighbors map[string]map[string]int, resolution float64) {
	minSize := int(5.0 / resolution)
	if minSize < 1 {
		minSize = 1
	}

	size := make(map[int]int)
	for _, id := range ids {
		size[comm[id]]++
	}

	for pass := 0; pass < 3; pass++ {
		changedAny := false
		for _, id := range ids {
			c := comm[id]
			if size[c] >= minSize {
				continue
			}
			counts := make(map[int]int)
			for neighbor, weight := range neighbors[id] {
				nc := comm[neighbor]
				if nc != c {
					counts[nc] += weight
				}
			}
			if len(counts) == 0 {
				continue
			}
			var candidates []int
			for nc := range counts {
				candidates = append(candidates, nc)
			}
			sort.Slice(candidates, func(i, j int) bool {
				if counts[candidates[i]] != counts[candidates[j]] {
					return counts[candidates[i]] > counts[candidates[j]]
				}
				return candidates[i] < candidates[j]
			})
			target := candidates[0]
			size[c]--
			size[target]++
			comm[id] = target
			changedAny = true
		}
		if !changedAny {
			break
		}
	}
}

// renumberCommunities reassigns community IDs to small, deterministic
// integers (0, 1, 2, ...), ordered by descending size then ascending
// smallest member ID, so output doesn't depend on internal label values.
func renumberCommunities(ids []string, comm map[string]int) {
	members := make(map[int][]string)
	for _, id := range ids {
		members[comm[id]] = append(members[comm[id]], id)
	}
	type group struct {
		old     int
		members []string
	}
	var groups []group
	for old, m := range members {
		sort.Strings(m)
		groups = append(groups, group{old: old, members: m})
	}
	sort.Slice(groups, func(i, j int) bool {
		if len(groups[i].members) != len(groups[j].members) {
			return len(groups[i].members) > len(groups[j].members)
		}
		return groups[i].members[0] < groups[j].members[0]
	})
	for newID, gr := range groups {
		for _, id := range gr.members {
			comm[id] = newID
		}
	}
}

var labelStopwords = map[string]bool{
	"internal": true, "pkg": true, "src": true, "cmd": true, "lib": true,
	"test": true, "tests": true, "main": true, "app": true, "com": true,
	"org": true, "io": true, "net": true, "util": true, "utils": true,
	"common": true, "core": true,
}

var nonAlnumRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// LabelCommunities sets Properties["community_label"] on every node to a
// human-readable label for its community (Node.Community must already be
// assigned), per graph-analytics' "Community labels generated"
// requirement. The label is the most common meaningful directory/package
// token across the community's nodes (e.g. "Auth"), falling back to the
// community's dominant node type (e.g. "Functions") when no such token
// stands out.
func LabelCommunities(g *graph.Graph) {
	for _, members := range g.Communities() {
		label := deriveCommunityLabel(members)
		for _, n := range members {
			if n.Properties == nil {
				n.Properties = make(map[string]any)
			}
			n.Properties[graph.PropertyCommunityLabel] = label
		}
	}
}

func deriveCommunityLabel(members []*graph.Node) string {
	tokenCounts := make(map[string]int)
	typeCounts := make(map[graph.NodeType]int)

	for _, n := range members {
		typeCounts[n.Type]++
		for _, tok := range pathTokens(n.File) {
			tokenCounts[tok]++
		}
		if name, _ := n.Properties["name"].(string); name != "" {
			for _, tok := range pathTokens(name) {
				tokenCounts[tok]++
			}
		}
	}

	bestToken, bestCount := "", 0
	for tok, count := range tokenCounts {
		if count > bestCount || (count == bestCount && tok < bestToken) {
			bestToken, bestCount = tok, count
		}
	}
	if bestToken != "" && bestCount*2 >= len(members) {
		return strings.ToUpper(bestToken[:1]) + bestToken[1:]
	}

	bestType, bestTypeCount := graph.NodeType(""), 0
	var types []graph.NodeType
	for t := range typeCounts {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	for _, t := range types {
		if typeCounts[t] > bestTypeCount {
			bestType, bestTypeCount = t, typeCounts[t]
		}
	}
	if bestType != "" {
		return string(bestType) + "s"
	}
	return "Misc"
}

// pathTokens splits a file path or dotted name into lowercase word tokens,
// dropping the final segment (the file/entity's own name, too specific to
// be a useful community label) and common stopwords.
func pathTokens(s string) []string {
	if s == "" {
		return nil
	}
	dir := filepath.ToSlash(filepath.Dir(s))
	var out []string
	for _, seg := range strings.Split(dir, "/") {
		for _, tok := range nonAlnumRe.Split(seg, -1) {
			tok = strings.ToLower(tok)
			if len(tok) < 3 || labelStopwords[tok] {
				continue
			}
			out = append(out, tok)
		}
	}
	return out
}
