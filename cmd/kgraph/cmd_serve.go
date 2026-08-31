package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"kgraph/internal/server"
	"kgraph/internal/store"
)

type serveMode uint8

const (
	serveHub serveMode = iota
	serveSingle
)

// selectServeMode centralizes the compatibility boundary: an entirely
// flag-free invocation opens the multi-project hub, while a picked or
// explicitly addressed project follows the original single-project path.
func selectServeMode(pick, repoSet, dbSet bool) serveMode {
	if pick || repoSet || dbSet {
		return serveSingle
	}
	return serveHub
}

// Variables make the OS-facing pieces replaceable in focused command tests;
// production always uses the store and terminal helpers directly.
var (
	discoverProjects = store.DiscoverProjects
	interactiveStdin = stdinIsTerminal
)

func newServeCmd() *cobra.Command {
	var addr string
	var pick bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve a live, browsable visualization of the graph over HTTP",
		Long: "Serve a live, browsable visualization of the graph over HTTP.\n\n" +
			"Without --repo/--db/--pick, serve runs in hub mode: it discovers every\n" +
			"built graph in the per-user cache directory, shows a project picker at /\n" +
			"and lazily serves each project's viewer under /p/<key>/.\n\n" +
			"With --repo (and optionally --db), serve runs in single-project mode\n" +
			"exactly as before, bound to that repository's database.\n\n" +
			"With --pick, serve lists the discovered built projects and lets you\n" +
			"choose one interactively, then serves it in single-project mode.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if pick {
				if cmd.Flags().Changed("repo") || cmd.Flags().Changed("db") {
					return fail(cmd, fmt.Errorf("serve: --pick cannot be combined with --repo or --db"))
				}
				repoPath, dbPath, err := pickProject(cmd)
				if err != nil {
					return fail(cmd, err)
				}
				// Route the selection through the existing single-project path.
				repoFlag, dbFlag = repoPath, dbPath
			}

			// Hub mode is the default; any explicit --repo/--db (or a pick)
			// keeps the traditional single-project behavior.
			if selectServeMode(pick, cmd.Flags().Changed("repo"), cmd.Flags().Changed("db")) == serveHub {
				return runHub(cmd, addr)
			}
			return runSingle(cmd, addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "localhost:7465", "address to listen on (host:port); binds to localhost by default")
	cmd.Flags().BoolVar(&pick, "pick", false, "interactively choose one of the built projects to serve (requires a terminal)")
	return cmd
}

// pickProject discovers every built graph and lets the user choose one on
// stdin, returning its recorded repo path and database path so the regular
// single-project flow can serve it. Non-TTY stdin fails with a clear
// pointer at --repo, per kgraph-cli's "Pick requires an interactive
// terminal" requirement.
func pickProject(cmd *cobra.Command) (repoPath, dbPath string, err error) {
	projects, err := discoverProjects()
	if err != nil {
		return "", "", err
	}

	// Only built databases are servable: never-built ones have no graph to
	// load, unreadable ones can't be opened at all.
	var selectable []store.ProjectInfo
	for _, p := range projects {
		if p.Status == store.StatusBuilt {
			selectable = append(selectable, p)
		}
	}
	if len(selectable) == 0 {
		return "", "", fmt.Errorf("no built graphs found — run `kgraph build` in a repository first")
	}
	if !interactiveStdin() {
		return "", "", fmt.Errorf("--pick needs an interactive terminal, but stdin is not a terminal — use `kgraph serve --repo <path>` instead")
	}

	items := make([]string, len(selectable))
	for i, p := range selectable {
		items[i] = fmt.Sprintf("%s — %s [%s]", displayName(p), p.RepoPath, freshnessLabel(store.ProjectFreshness(p)))
	}
	idx, err := chooseFromList(cmd.OutOrStdout(), cmd.InOrStdin(), "Built graphs:", items)
	if err != nil {
		return "", "", err
	}
	chosen := selectable[idx]
	return chosen.RepoPath, chosen.DBPath, nil
}

// runSingle is the pre-hub single-project serve: bound to one repo's
// database, failing with "run `kgraph build` first" when nothing was built,
// serving the unprefixed viewer/API surface at /.
func runSingle(cmd *cobra.Command, addr string) error {
	repoPath, dbPath, err := resolvePaths()
	if err != nil {
		return fail(cmd, err)
	}

	ro, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return fail(cmd, err)
	}
	defer ro.Close()

	if _, ok, err := ro.LastBuildAt(repoPath); err != nil {
		return fail(cmd, err)
	} else if !ok {
		return fail(cmd, fmt.Errorf("serve: no build recorded for %s — run `kgraph build` first", repoPath))
	}

	gs := server.NewGraphService()
	bc := server.NewBroadcaster()
	poller := server.NewPoller(ro, repoPath, gs, bc.Notify)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go poller.Run(ctx)

	httpSrv := &http.Server{Addr: addr, Handler: server.New(gs, bc).Handler()}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(cmd.OutOrStdout(), "kgraph serve: listening on http://%s (repo: %s)\n", addr, repoPath)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fail(cmd, err)
	}
	return nil
}

// runHub serves every discovered project: picker at /, per-project viewers
// and APIs under /p/<key>/ and /api/projects/<key>/, with runtimes created
// lazily on first visit and stopped on shutdown.
func runHub(cmd *cobra.Command, addr string) error {
	cacheRoot, err := store.CacheRoot()
	if err != nil {
		return fail(cmd, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hub := server.NewHub(ctx, cacheRoot)

	httpSrv := &http.Server{Addr: addr, Handler: hub.Handler()}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(cmd.OutOrStdout(), "kgraph serve: hub mode, listening on http://%s\n", addr)
	err = httpSrv.ListenAndServe()

	// Stop every per-project runtime before returning: cancel the context
	// (idempotent with the deferred stop) so pollers exit, then wait for
	// them and close their read-only stores.
	stop()
	hub.Shutdown()

	if err != nil && err != http.ErrServerClosed {
		return fail(cmd, err)
	}
	return nil
}
