package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kgraph/internal/graph"
	"kgraph/internal/store"
)

// buildHubFixture creates <cacheRoot>/<key>/graph.db as a built database
// with nodeCount nodes for repoPath.
func buildHubFixture(t *testing.T, cacheRoot, key, repoPath string, nodeCount int) {
	t.Helper()
	dbPath := filepath.Join(cacheRoot, key, "graph.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open(%s) error = %v", dbPath, err)
	}
	defer s.Close()

	g := graph.New()
	g.AddNode(&graph.Node{ID: "pkg:" + key, Type: graph.NodeTypePackage, Hash: "h1"})
	for i := 1; i < nodeCount; i++ {
		g.AddNode(&graph.Node{ID: fmt.Sprintf("%s.Fn%d()", key, i), Type: graph.NodeTypeFunction, Hash: "h"})
	}
	if _, err := s.SaveGraph(g); err != nil {
		t.Fatalf("SaveGraph() error = %v", err)
	}
	if err := s.SetLastCommit(repoPath, "commit-"+key); err != nil {
		t.Fatalf("SetLastCommit() error = %v", err)
	}
}

func waitForRuntimeSnapshot(t *testing.T, rt *ProjectRuntime) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if rt.GS.Snapshot() != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for a runtime's initial snapshot")
}

func TestRegistryLazyCreationAndUnknownKey(t *testing.T) {
	root := t.TempDir()
	buildHubFixture(t, root, "aaaaaaaaaaaaaaaa", "/repo/alpha", 2)

	reg := NewRuntimeRegistry(context.Background(), root)
	defer reg.Shutdown()

	if reg.Len() != 0 {
		t.Fatalf("a fresh registry must hold no runtimes, got %d", reg.Len())
	}

	rt, err := reg.Get("aaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	waitForRuntimeSnapshot(t, rt)
	if reg.Len() != 1 {
		t.Fatalf("expected exactly one runtime after first access, got %d", reg.Len())
	}

	// Second access returns the same runtime — no re-creation.
	rt2, err := reg.Get("aaaaaaaaaaaaaaaa")
	if err != nil || rt2 != rt {
		t.Fatalf("expected the same runtime on second Get, got %p vs %p (err=%v)", rt, rt2, err)
	}
	if reg.Len() != 1 {
		t.Fatalf("second Get must not create another runtime, got %d", reg.Len())
	}

	if _, err := reg.Get("ffffffffffffffff"); err == nil {
		t.Fatal("expected an error for an unknown key")
	} else if !isUnknownProject(err) {
		t.Fatalf("expected ErrUnknownProject, got %v", err)
	}
}

func isUnknownProject(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unknown project")
}

func TestRegistryRejectsUnreadableProject(t *testing.T) {
	root := t.TempDir()
	corrupt := filepath.Join(root, "cccccccccccccccc", "graph.db")
	if err := os.MkdirAll(filepath.Dir(corrupt), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(corrupt, []byte("garbage"), 0o644); err != nil {
		t.Fatalf("writing corrupt fixture: %v", err)
	}

	reg := NewRuntimeRegistry(context.Background(), root)
	defer reg.Shutdown()

	if _, err := reg.Get("cccccccccccccccc"); err == nil {
		t.Fatal("expected an error for an unreadable database")
	}
}

func TestRegistryRejectsNewRuntimeAfterShutdown(t *testing.T) {
	root := t.TempDir()
	buildHubFixture(t, root, "aaaaaaaaaaaaaaaa", "/repo/alpha", 1)

	reg := NewRuntimeRegistry(context.Background(), root)
	reg.Shutdown()

	if _, err := reg.Get("aaaaaaaaaaaaaaaa"); !errors.Is(err, ErrRegistryClosed) {
		t.Fatalf("Get() after Shutdown() error = %v, want ErrRegistryClosed", err)
	}
}

// sseEvents spawns one reader goroutine that forwards every SSE event name
// from body into the returned channel (closed when the body ends). One
// persistent reader avoids concurrent reads on the same body across
// successive assertions.
func sseEvents(body io.Reader) <-chan string {
	ch := make(chan string, 8)
	go func() {
		defer close(ch)
		br := bufio.NewReader(body)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "event: ") {
				ch <- strings.TrimSpace(strings.TrimPrefix(line, "event: "))
			}
		}
	}()
	return ch
}

// TestPerProjectSSEIsolation proves a rebuild of project B emits no event on
// project A's stream: each runtime owns its own Broadcaster, so notifying B
// can never wake A's subscribers — and A's own rebuild still does.
func TestPerProjectSSEIsolation(t *testing.T) {
	root := t.TempDir()
	buildHubFixture(t, root, "aaaaaaaaaaaaaaaa", "/repo/alpha", 1)
	buildHubFixture(t, root, "bbbbbbbbbbbbbbbb", "/repo/beta", 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub := NewHub(ctx, root)
	defer hub.Shutdown()

	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	// Create project A's runtime first and wait for its initial load: the
	// poller notifies its own broadcaster on that first load, and that
	// notification must not be mistaken for a cross-project event below.
	rtA, err := hub.Registry().Get("aaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("Get(A) error = %v", err)
	}
	waitForRuntimeSnapshot(t, rtA)

	// Connect to project A's stream; once the request returns, A's
	// subscription is registered (Subscribe precedes the header flush).
	res, err := http.Get(srv.URL + "/api/projects/aaaaaaaaaaaaaaaa/events")
	if err != nil {
		t.Fatalf("GET A events: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for A's event stream, got %d", res.StatusCode)
	}

	rtB, err := hub.Registry().Get("bbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("Get(B) error = %v", err)
	}
	events := sseEvents(res.Body)

	// Rebuild-style notification on B only: A's stream must stay silent.
	rtB.BC.Notify()
	select {
	case ev := <-events:
		t.Fatalf("project A's stream must not receive project B's update, got event %q", ev)
	case <-time.After(300 * time.Millisecond):
	}

	// A's own update still arrives on A's stream.
	rtA.BC.Notify()
	select {
	case ev := <-events:
		if ev != "graph-updated" {
			t.Fatalf("expected graph-updated on A's own stream, got %q", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected graph-updated on A's own stream, got nothing")
	}
}

func TestProjectsEndpointWithoutLoadingGraphs(t *testing.T) {
	root := t.TempDir()
	buildHubFixture(t, root, "aaaaaaaaaaaaaaaa", "/repo/alpha", 2)
	buildHubFixture(t, root, "bbbbbbbbbbbbbbbb", "/repo/beta", 3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub := NewHub(ctx, root)
	defer hub.Shutdown()

	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/projects")
	if err != nil {
		t.Fatalf("GET /api/projects: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var dtos []ProjectDTO
	if err := json.NewDecoder(res.Body).Decode(&dtos); err != nil {
		t.Fatalf("decoding /api/projects: %v", err)
	}
	if len(dtos) != 2 {
		t.Fatalf("expected 2 projects, got %+v", dtos)
	}
	byKey := map[string]ProjectDTO{}
	for _, d := range dtos {
		byKey[d.Key] = d
	}
	a := byKey["aaaaaaaaaaaaaaaa"]
	if a.Status != "built" || a.Name != "alpha" || a.NodeCount != 2 {
		t.Errorf("bad DTO for alpha: %+v", a)
	}
	if byKey["bbbbbbbbbbbbbbbb"].NodeCount != 3 {
		t.Errorf("bad DTO for beta: %+v", byKey["bbbbbbbbbbbbbbbb"])
	}

	// The whole point of lazy loading: listing projects opened no runtimes.
	if hub.Registry().Len() != 0 {
		t.Fatalf("/api/projects must not load any graph, but %d runtime(s) exist", hub.Registry().Len())
	}
}

func TestHubUnknownProjectKeyYieldsJSON404(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub := NewHub(ctx, t.TempDir())
	defer hub.Shutdown()

	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/projects/ffffffffffffffff/graph")
	if err != nil {
		t.Fatalf("GET unknown project graph: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown project, got %d", res.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("expected a JSON error body, got decode error: %v", err)
	}
	if !strings.Contains(body["error"], "unknown project") {
		t.Errorf("expected a clear unknown-project message, got %q", body["error"])
	}
}

func TestHubViewerPageAndRedirect(t *testing.T) {
	root := t.TempDir()
	buildHubFixture(t, root, "aaaaaaaaaaaaaaaa", "/repo/alpha", 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub := NewHub(ctx, root)
	defer hub.Shutdown()

	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	// /p/<key> redirects to /p/<key>/.
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := client.Get(srv.URL + "/p/aaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("GET /p/<key>: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("expected 301 for /p/<key>, got %d", res.StatusCode)
	}
	if loc := res.Header.Get("Location"); loc != "/p/aaaaaaaaaaaaaaaa/" {
		t.Errorf("expected redirect to /p/aaaaaaaaaaaaaaaa/, got %q", loc)
	}

	// /p/<key>/ renders the viewer configured for that project.
	res, err = http.Get(srv.URL + "/p/aaaaaaaaaaaaaaaa/")
	if err != nil {
		t.Fatalf("GET /p/<key>/: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for viewer page, got %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	html := string(body)
	if !strings.Contains(html, "/api/projects/aaaaaaaaaaaaaaaa") {
		t.Errorf("viewer page must be configured with the project's API base:\n%s", html)
	}
	if !strings.Contains(html, `"mode":"hub"`) || !strings.Contains(html, `"page":"viewer"`) {
		t.Errorf("hub viewer must retain hub mode and identify itself as a viewer:\n%s", html)
	}
	if !strings.Contains(html, `id="home-link"`) {
		t.Errorf("hub-mode viewer must offer a way back to the picker:\n%s", html)
	}

	// Unknown project page: clear 404, not an empty viewer.
	res2, err := http.Get(srv.URL + "/p/ffffffffffffffff/")
	if err != nil {
		t.Fatalf("GET unknown viewer page: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown project page, got %d", res2.StatusCode)
	}
}

func TestHubRootServesPicker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub := NewHub(ctx, t.TempDir())
	defer hub.Shutdown()

	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	html := string(body)

	if !strings.Contains(html, `id="picker"`) {
		t.Errorf("hub root must render the picker view:\n%s", html)
	}
	if strings.Contains(html, `id="graph"`) {
		t.Errorf("hub root must not render a graph canvas:\n%s", html)
	}
	if !strings.Contains(html, `"mode":"hub"`) {
		t.Errorf("hub root must inject hub bootstrap config:\n%s", html)
	}
	if !strings.Contains(html, `"page":"picker"`) {
		t.Errorf("hub root must inject picker bootstrap config:\n%s", html)
	}
}

func TestHubPerProjectGraphEndpoint(t *testing.T) {
	root := t.TempDir()
	buildHubFixture(t, root, "aaaaaaaaaaaaaaaa", "/repo/alpha", 3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub := NewHub(ctx, root)
	defer hub.Shutdown()

	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	// Poll until the lazily-created runtime has loaded, then fetch the graph.
	var dto GraphDTO
	deadline := time.Now().Add(3 * time.Second)
	for {
		res, err := http.Get(srv.URL + "/api/projects/aaaaaaaaaaaaaaaa/graph")
		if err != nil {
			t.Fatalf("GET project graph: %v", err)
		}
		if res.StatusCode == http.StatusServiceUnavailable {
			res.Body.Close()
			if time.Now().After(deadline) {
				t.Fatal("project graph never became ready")
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}
		err = json.NewDecoder(res.Body).Decode(&dto)
		res.Body.Close()
		if err != nil {
			t.Fatalf("decoding graph: %v", err)
		}
		break
	}
	if len(dto.Nodes) == 0 {
		t.Errorf("expected the project's own graph nodes, got %+v", dto)
	}
}

// TestSingleModeRoutesUnchanged guards the backward-compatibility
// requirement: with a pinned GraphService the unprefixed surface responds
// exactly as pre-hub (covered in depth by api_test.go; here just the shape
// of the single-mode index with its injected config).
func TestSingleModeIndexKeepsUnprefixedSurface(t *testing.T) {
	srv := newTestServer(buildAPIFixture())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	html := string(body)

	if !strings.Contains(html, `"mode":"single"`) {
		t.Errorf("single-mode index must carry single bootstrap config:\n%s", html)
	}
	if !strings.Contains(html, `"page":"viewer"`) {
		t.Errorf("single-mode index must identify itself as a viewer:\n%s", html)
	}
	if !strings.Contains(html, `id="graph"`) {
		t.Errorf("single-mode index must render the viewer:\n%s", html)
	}
	if strings.Contains(html, `id="home-link"`) {
		t.Errorf("single-project mode must not show a picker/home link:\n%s", html)
	}
	if strings.Contains(html, `id="picker"`) {
		t.Errorf("single-project mode must not render the picker:\n%s", html)
	}
}
