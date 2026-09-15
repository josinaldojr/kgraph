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
// values below 1.0 favor fewer, larger ones.
const DefaultResolution = 1.0

// maxLouvainIterations bounds both the local-moving phase's per-level pass
// count and the outer aggregation-level loop, so a pathological graph can't
// spin forever; well-behaved code graphs converge in a handful of either.
const maxLouvainIterations = 20

// DetectCommunities partitions g into communities using Louvain modularity
// optimization (Blondel et al.): repeated local moves of each node into
// whichever neighboring community most increases modularity, then
// aggregation (each community collapses into one node, edges reweighted),
// repeated until no further gain — see design.md Decision 4. resolution is
// Louvain's own resolution parameter (γ): higher resolution favors more,
// smaller communities; lower resolution favors fewer, larger ones. Node
// iteration order is fixed (sorted by ID) and ties in modularity gain
// prefer, in order: staying in the current community, then the lowest
// community ID — so output is deterministic given the same graph and
// resolution. Sets Properties["community"] on every node to a small
// integer, densest community first.
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

	comm := louvain(ids, neighbors, resolution)
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
// Louvain's base-level edge weights only care about connection count, not
// edge semantics).
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

// louvain runs the Louvain modularity-optimization algorithm over the
// undirected weighted graph described by ids (already sorted) and
// neighbors (neighbor ID -> edge weight, per node), returning each node
// ID's final community assignment as an arbitrary (not yet renumbered)
// integer label.
func louvain(ids []string, neighbors map[string]map[string]int, resolution float64) map[string]int {
	n := len(ids)
	idxOf := make(map[string]int, n)
	for i, id := range ids {
		idxOf[id] = i
	}

	adj := make([]map[int]float64, n)
	for i := range adj {
		adj[i] = make(map[int]float64)
	}
	for id, nbs := range neighbors {
		i := idxOf[id]
		for nb, w := range nbs {
			j, ok := idxOf[nb]
			if !ok || i == j {
				continue
			}
			adj[i][j] += float64(w)
		}
	}

	degree := make([]float64, n)
	m := 0.0
	for i := range adj {
		for _, w := range adj[i] {
			degree[i] += w
		}
		m += degree[i]
	}
	m /= 2

	out := make(map[string]int, n)
	if m == 0 {
		// No edges at all: every node is its own community, and there is
		// no gain the local-moving phase could ever find.
		for i, id := range ids {
			out[id] = i
		}
		return out
	}

	// owner[i] tracks, for original node i, the index of the current-level
	// supernode it currently belongs to — composed across aggregation
	// levels so the final value is i's top-level community.
	owner := make([]int, n)
	for i := range owner {
		owner[i] = i
	}

	curN, curAdj, curDeg := n, adj, degree
	for level := 0; level < maxLouvainIterations; level++ {
		comm, moved := localMoving(curAdj, curDeg, m, resolution)
		if !moved {
			break
		}
		newN, newAdj, newDeg, relabel := aggregate(curAdj, comm, curN)
		if newN == curN {
			break
		}
		for i := range owner {
			owner[i] = relabel[comm[owner[i]]]
		}
		curN, curAdj, curDeg = newN, newAdj, newDeg
	}

	for i, id := range ids {
		out[id] = owner[i]
	}
	return out
}

// localMoving runs Louvain's local-moving phase to a fixed point (or until
// maxLouvainIterations passes) on the graph described by adj (symmetric
// neighbor-weight maps, no self entries) and degree (each node's weighted
// degree). m is the base graph's total edge weight, fixed across every
// aggregation level per Louvain's modularity definition. Node evaluation
// order is always 0..n-1 (the caller's sorted-ID index space), and a tie in
// modularity gain prefers, in order: the node's current community, then the
// lowest community ID — the same deterministic tie-break at every level.
// Returns the resulting community assignment (community IDs are node
// indices, not yet renumbered) and whether any node ever moved from its
// initial singleton assignment.
func localMoving(adj []map[int]float64, degree []float64, m, resolution float64) ([]int, bool) {
	n := len(adj)
	comm := make([]int, n)
	commTotal := make([]float64, n) // Σ_tot per community, indexed by community ID (== node ID initially)
	for i := range comm {
		comm[i] = i
		commTotal[i] = degree[i]
	}

	movedAny := false
	for iter := 0; iter < maxLouvainIterations; iter++ {
		changed := false
		for i := 0; i < n; i++ {
			current := comm[i]
			commTotal[current] -= degree[i]

			kIn := make(map[int]float64, len(adj[i])+1)
			for j, w := range adj[i] {
				kIn[comm[j]] += w
			}

			candidates := make([]int, 0, len(kIn)+1)
			for c := range kIn {
				if c != current {
					candidates = append(candidates, c)
				}
			}
			sort.Ints(candidates)

			best := current
			bestGain := kIn[current] - resolution*degree[i]*commTotal[current]/(2*m)
			for _, c := range candidates {
				gain := kIn[c] - resolution*degree[i]*commTotal[c]/(2*m)
				if gain > bestGain {
					best, bestGain = c, gain
				}
			}

			commTotal[best] += degree[i]
			comm[i] = best
			if best != current {
				changed = true
				movedAny = true
			}
		}
		if !changed {
			break
		}
	}
	return comm, movedAny
}

// aggregate collapses adj (n nodes) into a new graph where each node is one
// community from comm (comm[i] in [0,n)): the weight between two
// communities is the sum of their members' inter-community edge weights,
// and a community's self-loop weight (its members' intra-community edges)
// is folded into its degree. relabel maps every community label used in
// comm to a small deterministic index 0..newN-1, ordered by ascending
// original label, so the mapping — and therefore the aggregated graph
// itself — never depends on map iteration order.
func aggregate(adj []map[int]float64, comm []int, n int) (newN int, newAdj []map[int]float64, newDegree []float64, relabel []int) {
	used := make(map[int]bool)
	for _, c := range comm {
		used[c] = true
	}
	labels := make([]int, 0, len(used))
	for c := range used {
		labels = append(labels, c)
	}
	sort.Ints(labels)

	relabel = make([]int, n)
	for newIdx, old := range labels {
		relabel[old] = newIdx
	}
	newN = len(labels)

	newAdj = make([]map[int]float64, newN)
	for i := range newAdj {
		newAdj[i] = make(map[int]float64)
	}
	selfLoop := make([]float64, newN)
	for i := 0; i < n; i++ {
		ci := relabel[comm[i]]
		for j, w := range adj[i] {
			if j <= i {
				continue // each undirected pair counted once, from its lower-index side
			}
			cj := relabel[comm[j]]
			if ci == cj {
				selfLoop[ci] += w
			} else {
				newAdj[ci][cj] += w
				newAdj[cj][ci] += w
			}
		}
	}

	newDegree = make([]float64, newN)
	for i := 0; i < newN; i++ {
		deg := 2 * selfLoop[i]
		for _, w := range newAdj[i] {
			deg += w
		}
		newDegree[i] = deg
	}
	return newN, newAdj, newDegree, relabel
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
