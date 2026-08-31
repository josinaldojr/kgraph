// Package server implements `kgraph serve`: a long-running, read-only HTTP
// server that visualizes the graph as an Obsidian-style force-directed
// diagram and stays live as a separate `kgraph build`/`kgraph update`
// process writes to the same database — see openspec/changes/
// kgraph-web-viewer for the full design.
package server

import (
	"fmt"
	"time"

	"kgraph/internal/graph"
	"kgraph/internal/store"
	"kgraph/internal/summarizer"
)

// Snapshot is one immutable, fully-loaded view of the graph plus its
// summaries at every level — what GraphService atomically swaps in each
// time the poller (poller.go) detects a new build/update. Immutable so
// concurrent HTTP handlers never need to lock around reading it.
type Snapshot struct {
	Graph           *graph.Graph
	NodeSummaries   map[string]store.SummaryRecord
	FileSummaries   map[string]store.SummaryRecord
	ModuleSummaries map[string]store.SummaryRecord
	LoadedAt        time.Time
}

// LoadSnapshot reads the full graph and every summary level from ro. This
// is the same cost class as any one-shot CLI command's loadGraphAndStore —
// see design.md's risk note on reload cost.
func LoadSnapshot(ro *store.ReadOnlyStore) (*Snapshot, error) {
	g, err := ro.LoadGraph()
	if err != nil {
		return nil, fmt.Errorf("server: loading graph: %w", err)
	}
	nodeSums, err := ro.SummariesByLevel(summarizer.LevelNode)
	if err != nil {
		return nil, fmt.Errorf("server: loading node summaries: %w", err)
	}
	fileSums, err := ro.SummariesByLevel(summarizer.LevelFile)
	if err != nil {
		return nil, fmt.Errorf("server: loading file summaries: %w", err)
	}
	moduleSums, err := ro.SummariesByLevel(summarizer.LevelModule)
	if err != nil {
		return nil, fmt.Errorf("server: loading module summaries: %w", err)
	}
	return &Snapshot{
		Graph: g, NodeSummaries: nodeSums, FileSummaries: fileSums, ModuleSummaries: moduleSums,
		LoadedAt: time.Now(),
	}, nil
}
