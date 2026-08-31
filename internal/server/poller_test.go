package server

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"kgraph/internal/graph"
	"kgraph/internal/store"
)

func waitForSnapshot(t *testing.T, gs *GraphService) *Snapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if snap := gs.Snapshot(); snap != nil {
			return snap
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for an initial snapshot")
	return nil
}

func TestPollerLoadsInitialSnapshotAndNotifiesOnChange(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "graph.db")
	rw, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer rw.Close()

	g := graph.New()
	g.AddNode(&graph.Node{ID: "pkg:demo", Type: graph.NodeTypePackage, Hash: "h1"})
	if _, err := rw.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph() error = %v", err)
	}
	if err := rw.SetLastCommit("/repo", "c1"); err != nil {
		t.Fatalf("SetLastCommit() error = %v", err)
	}

	ro, err := store.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("OpenReadOnly() error = %v", err)
	}
	defer ro.Close()

	gs := NewGraphService()
	notified := make(chan struct{}, 10)
	p := NewPoller(ro, "/repo", gs, func() { notified <- struct{}{} })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go p.Run(ctx)

	if snap := waitForSnapshot(t, gs); snap.Graph.NodeCount() != 1 {
		t.Fatalf("expected initial snapshot with 1 node, got %d", snap.Graph.NodeCount())
	}
	select {
	case <-notified:
	case <-time.After(time.Second):
		t.Fatal("expected an initial update notification")
	}

	select {
	case <-notified:
		t.Fatal("expected no notification when build_meta hasn't changed")
	case <-time.After(pollInterval + 500*time.Millisecond):
	}

	g2 := graph.New()
	g2.AddNode(&graph.Node{ID: "pkg:demo", Type: graph.NodeTypePackage, Hash: "h1"})
	g2.AddNode(&graph.Node{ID: "demo.Foo()", Type: graph.NodeTypeFunction, Hash: "h2"})
	if _, err := rw.SaveGraph(g2); err != nil {
		t.Fatalf("SaveGraph() error = %v", err)
	}
	if err := rw.SetLastCommit("/repo", "c2"); err != nil {
		t.Fatalf("SetLastCommit() error = %v", err)
	}

	select {
	case <-notified:
	case <-time.After(pollInterval*2 + time.Second):
		t.Fatal("expected a notification after build_meta changed")
	}
	if got := gs.Snapshot().Graph.NodeCount(); got != 2 {
		t.Fatalf("expected reloaded snapshot with 2 nodes, got %d", got)
	}
}

func TestBroadcasterCoalescesNotifications(t *testing.T) {
	b := NewBroadcaster()
	ch, cancel := b.Subscribe()
	defer cancel()

	b.Notify()
	b.Notify()
	b.Notify()

	select {
	case <-ch:
	default:
		t.Fatal("expected at least one queued notification")
	}
	select {
	case <-ch:
		t.Fatal("expected repeated Notify calls to coalesce into a single pending notification")
	default:
	}
}
