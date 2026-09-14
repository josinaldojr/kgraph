package analytics

import "kgraph/internal/graph"

// AnalyzeGraph runs the full analytics stage over g: degree computation,
// god-node detection (top godNodeCount nodes by degree, or
// DefaultGodNodeCount if <= 0), and community detection/labeling at
// resolution (DefaultResolution if <= 0, matching DetectCommunities' own
// fallback). It's the entry point the build pipeline (internal/build) and
// `kgraph analyze` call.
func AnalyzeGraph(g *graph.Graph, godNodeCount int, resolution float64) error {
	if godNodeCount <= 0 {
		godNodeCount = DefaultGodNodeCount
	}
	ComputeDegree(g)
	DetectGodNodes(g, godNodeCount)
	DetectCommunities(g, resolution)
	LabelCommunities(g)
	return nil
}
