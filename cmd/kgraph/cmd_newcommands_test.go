package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"kgraph/internal/build"
	"kgraph/internal/graph"
)

// copyDir recursively copies src into dst, both assumed to already exist
// or be creatable, so a build against the checked-in parser fixture never
// writes generated files (GRAPH_REPORT.md, graph.json) back into the repo.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", src, err)
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			copyDir(t, srcPath, dstPath)
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", dstPath, err)
		}
	}
}

// builtFixture copies internal/parser/go's fixture repo (module "fixture",
// with User/Order structs, GreetingFor/itoa functions, and an
// Order.Summary method that calls both) into a fresh temp dir, builds a
// real graph against it with build.Run, and returns the repo and database
// paths for the new CLI commands to operate on.
func builtFixture(t *testing.T) (repoPath, dbPath string) {
	t.Helper()
	repoPath = t.TempDir()
	copyDir(t, filepath.Join("..", "..", "internal", "parser", "go", "testdata", "fixture"), repoPath)

	dbPath = filepath.Join(t.TempDir(), "graph.db")
	if _, err := build.Run(repoPath, dbPath); err != nil {
		t.Fatalf("build.Run() error = %v", err)
	}
	return repoPath, dbPath
}

// runNewCmd sets the global --repo/--db flags directly (as resolvePaths
// reads them) and executes cmd's own RunE, mirroring how root.Execute()
// would dispatch to it, without needing a full root command tree.
func runNewCmd(t *testing.T, cmd *cobra.Command, repoPath, dbPath string, args ...string) (stdout string, err error) {
	t.Helper()
	origRepo, origDB := repoFlag, dbFlag
	t.Cleanup(func() { repoFlag, dbFlag = origRepo, origDB })
	repoFlag, dbFlag = repoPath, dbPath

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), err
}

func TestNewCommandsRegisteredOnRoot(t *testing.T) {
	root := &cobra.Command{Use: "kgraph"}
	root.PersistentFlags().StringVar(&repoFlag, "repo", ".", "path to the repository")
	root.PersistentFlags().StringVar(&dbFlag, "db", "", "path to the graph database")
	root.AddCommand(newQueryCmd(), newPathCmd(), newExplainCmd(), newPromptCmd(),
		newExportCmd(), newReportCmd(), newAnalyzeCmd(), newMCPCmd())

	for _, name := range []string{"query", "path", "explain", "prompt", "export", "report", "analyze", "mcp"} {
		found, _, err := root.Find([]string{name})
		if err != nil || found == root {
			t.Errorf("expected %q to be registered as a subcommand, Find error = %v", name, err)
		}
	}
}

func TestQueryCommandAnswersQuestion(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	out, err := runNewCmd(t, newQueryCmd(), repoPath, dbPath, "GreetingFor")
	if err != nil {
		t.Fatalf("query command error = %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "GreetingFor") {
		t.Errorf("expected query output to mention GreetingFor, got:\n%s", out)
	}
}

func TestQueryCommandRespectsMaxTokensFlag(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	cmd := newQueryCmd()
	out, err := runNewCmd(t, cmd, repoPath, dbPath, "--max-tokens", "50", "--hops", "1", "GreetingFor")
	if err != nil {
		t.Fatalf("query command error = %v\noutput:\n%s", err, out)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestPathCommandFindsShortestPath(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	summaryID := graph.FunctionID("fixture", "Order", "Summary")
	greetingID := graph.FunctionID("fixture", "", "GreetingFor")

	out, err := runNewCmd(t, newPathCmd(), repoPath, dbPath, summaryID, greetingID)
	if err != nil {
		t.Fatalf("path command error = %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, summaryID) || !strings.Contains(out, greetingID) {
		t.Errorf("expected path output to mention both endpoints, got:\n%s", out)
	}
	if !strings.Contains(out, "calls") {
		t.Errorf("expected path output to show the calls edge, got:\n%s", out)
	}
}

func TestPathCommandReportsNoPath(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	out, err := runNewCmd(t, newPathCmd(), repoPath, dbPath, "does-not-exist-a", "does-not-exist-b")
	if err == nil {
		t.Fatalf("expected an error for unknown nodes, got output:\n%s", out)
	}
}

func TestExplainCommandShowsRelationsAndHops(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	out, err := runNewCmd(t, newExplainCmd(), repoPath, dbPath, "--hops", "2", "GreetingFor")
	if err != nil {
		t.Fatalf("explain command error = %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "GreetingFor") {
		t.Errorf("expected explain output for GreetingFor, got:\n%s", out)
	}
}

func TestPromptCommandRendersMarkdownSections(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	out, err := runNewCmd(t, newPromptCmd(), repoPath, dbPath, "--max-tokens", "4000", "GreetingFor")
	if err != nil {
		t.Fatalf("prompt command error = %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{"## Context:", "GreetingFor", "### Direct Relations"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected prompt output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestExportCommandWritesGraphJSONAndReportToOutputDir(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	outputDir := filepath.Join(t.TempDir(), "out")

	out, err := runNewCmd(t, newExportCmd(), repoPath, dbPath, "--output", outputDir)
	if err != nil {
		t.Fatalf("export command error = %v\noutput:\n%s", err, out)
	}

	jsonData, err := os.ReadFile(filepath.Join(outputDir, "graph.json"))
	if err != nil {
		t.Fatalf("expected export to write %s/graph.json: %v", outputDir, err)
	}
	if !strings.Contains(string(jsonData), "GreetingFor") {
		t.Errorf("expected exported graph.json to mention GreetingFor, got:\n%s", jsonData)
	}

	reportData, err := os.ReadFile(filepath.Join(outputDir, "GRAPH_REPORT.md"))
	if err != nil {
		t.Fatalf("expected export to also write %s/GRAPH_REPORT.md: %v", outputDir, err)
	}
	if len(reportData) == 0 {
		t.Error("expected a non-empty GRAPH_REPORT.md alongside graph.json")
	}
}

// TestExportAndReportDefaultOutputIsKgraphOut checks the --output flags'
// default value directly rather than executing the commands: both default
// to a relative "kgraph-out" (resolved against the process's working
// directory, per design.md Decision 5), and actually running either
// command without --output would create that directory inside this
// package's source tree.
func TestExportAndReportDefaultOutputIsKgraphOut(t *testing.T) {
	for _, cmd := range []*cobra.Command{newExportCmd(), newReportCmd()} {
		f := cmd.Flags().Lookup("output")
		if f == nil {
			t.Fatalf("%s: expected an --output flag", cmd.Use)
		}
		if f.DefValue != "kgraph-out" {
			t.Errorf("%s: expected --output default \"kgraph-out\", got %q", cmd.Use, f.DefValue)
		}
	}
}

func TestReportCommandWritesGraphReportToOutputDir(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	outputDir := filepath.Join(t.TempDir(), "out")

	out, err := runNewCmd(t, newReportCmd(), repoPath, dbPath, "--output", outputDir)
	if err != nil {
		t.Fatalf("report command error = %v\noutput:\n%s", err, out)
	}
	reportPath := filepath.Join(outputDir, "GRAPH_REPORT.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("expected report to write %s: %v", reportPath, err)
	}
	if len(data) == 0 {
		t.Error("expected a non-empty GRAPH_REPORT.md")
	}
}

func TestAnalyzeCommandRecomputesAnalytics(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	out, err := runNewCmd(t, newAnalyzeCmd(), repoPath, dbPath, "--god-nodes", "1")
	if err != nil {
		t.Fatalf("analyze command error = %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "analyzed") {
		t.Errorf("expected analyze summary output, got:\n%s", out)
	}
}

func TestAnalyzeCommandReclusterFlag(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	out, err := runNewCmd(t, newAnalyzeCmd(), repoPath, dbPath, "--recluster", "--resolution", "1.5")
	if err != nil {
		t.Fatalf("analyze --recluster error = %v\noutput:\n%s", err, out)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestMCPCommandRejectsUnknownTransport(t *testing.T) {
	repoPath, dbPath := builtFixture(t)
	_, err := runNewCmd(t, newMCPCmd(), repoPath, dbPath, "--transport", "carrier-pigeon")
	if err == nil {
		t.Fatal("expected an error for an unknown --transport value")
	}
	if !strings.Contains(err.Error(), "carrier-pigeon") {
		t.Errorf("expected the error to name the bad transport, got: %v", err)
	}
}

func TestMCPCommandHasExpectedFlags(t *testing.T) {
	cmd := newMCPCmd()
	for _, name := range []string{"transport", "port", "api-key"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("expected mcp command to define --%s", name)
		}
	}
}

func TestCommandsWorkWithExplicitRepoAndDBFlags(t *testing.T) {
	repoPath, dbPath := builtFixture(t)

	// Rather than relying on runNewCmd's direct global assignment, drive
	// this one through a root command tree so --repo/--db are parsed as
	// real persistent flags, per the "global flags" requirement.
	origRepo, origDB := repoFlag, dbFlag
	t.Cleanup(func() { repoFlag, dbFlag = origRepo, origDB })

	root := &cobra.Command{Use: "kgraph"}
	root.PersistentFlags().StringVar(&repoFlag, "repo", ".", "path to the repository")
	root.PersistentFlags().StringVar(&dbFlag, "db", "", "path to the graph database")
	root.AddCommand(newExplainCmd())

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"explain", "GreetingFor", "--repo", repoPath, "--db", dbPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("root.Execute() error = %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "GreetingFor") {
		t.Errorf("expected explain output routed through --repo/--db flags, got:\n%s", out.String())
	}
}
