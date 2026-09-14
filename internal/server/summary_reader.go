package server

import (
	"fmt"

	"github.com/josinaldojr/kgraph/internal/store"
	"github.com/josinaldojr/kgraph/internal/summarizer"
)

// snapshotSummaryReader adapts a Snapshot's already-loaded summary maps to
// summarizer.SummaryReader, so /api/search runs SearchNodes against memory
// instead of issuing a fresh database query per request — keeping with
// design.md Decision 3 ("only the poller touches the database on a
// timer").
type snapshotSummaryReader struct {
	snap *Snapshot
}

func (r snapshotSummaryReader) SummariesByLevel(level string) (map[string]store.SummaryRecord, error) {
	switch level {
	case summarizer.LevelNode:
		return r.snap.NodeSummaries, nil
	case summarizer.LevelFile:
		return r.snap.FileSummaries, nil
	case summarizer.LevelModule:
		return r.snap.ModuleSummaries, nil
	default:
		return nil, fmt.Errorf("server: unknown summary level %q", level)
	}
}
