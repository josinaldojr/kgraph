// Package summarizer implements provider-driven summarization: kgraph
// never calls an LLM API itself (see design.md Decision 6). Instead,
// PendingSummaries exports nodes/files/modules needing a summary as plain
// data, and ApplySummaries stores back whatever an AI provider (already
// driving the session) wrote for them, validated by content hash.
package summarizer

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"kgraph/internal/graph"
	"kgraph/internal/store"
)

// PendingSummary is one node/file/module awaiting a summary.
type PendingSummary struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Signature string `json:"signature,omitempty"`
	File      string `json:"file,omitempty"`
	LineStart int    `json:"line_start,omitempty"`
	LineEnd   int    `json:"line_end,omitempty"`
	Hash      string `json:"hash"`
	Source    string `json:"source"`
}

const (
	LevelNode   = "node"
	LevelFile   = "file"
	LevelModule = "module"
)

// summarizableTypes are the node types that get their own summary. Field
// and Column are deliberately excluded — described only as part of their
// owning Struct/Table's summary, to avoid one-line-per-field noise (see
// proposal.md's scope decision).
var summarizableTypes = map[graph.NodeType]bool{
	graph.NodeTypeStruct:             true,
	graph.NodeTypeInterface:          true,
	graph.NodeTypeFunction:           true,
	graph.NodeTypeTable:              true,
	graph.NodeTypeEndpoint:           true,
	graph.NodeTypeExternalDependency: true,
}

// hasOwnSourceSpan reports whether a node type's source text comes from a
// file/line span read off disk (readSourceLines) versus being synthesized
// from the node's own properties and edges (sourceTextFor).
func hasOwnSourceSpan(t graph.NodeType) bool {
	switch t {
	case graph.NodeTypeStruct, graph.NodeTypeInterface, graph.NodeTypeFunction:
		return true
	default:
		return false
	}
}

// PendingSummaries returns up to limit (0 = unlimited) nodes/files/modules
// at the given level lacking a current-hash summary.
func PendingSummaries(g *graph.Graph, s *store.Store, level string, limit int) ([]PendingSummary, error) {
	switch level {
	case LevelNode:
		return pendingNodes(g, s, limit)
	case LevelFile:
		return pendingFiles(g, s, limit)
	case LevelModule:
		return pendingModules(g, s, limit)
	default:
		return nil, fmt.Errorf("summarizer: unknown level %q", level)
	}
}

func pendingNodes(g *graph.Graph, s *store.Store, limit int) ([]PendingSummary, error) {
	nodes := g.Nodes()
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	var out []PendingSummary
	for _, n := range nodes {
		if !summarizableTypes[n.Type] {
			continue
		}
		rec, ok, err := s.GetSummary(n.ID, LevelNode)
		if err != nil {
			return nil, err
		}
		if ok && rec.Hash == n.Hash && !rec.Stale {
			continue
		}

		var source string
		if hasOwnSourceSpan(n.Type) {
			source, err = readSourceLines(n.File, n.LineStart, n.LineEnd)
			if err != nil {
				source = "" // best-effort: still list the node, just without source text
			}
		} else {
			source = sourceTextFor(g, n)
		}

		out = append(out, PendingSummary{
			ID: n.ID, Type: string(n.Type), Signature: n.Signature,
			File: n.File, LineStart: n.LineStart, LineEnd: n.LineEnd,
			Hash: n.Hash, Source: source,
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// pendingFiles derives a file's "content" from its children's summaries,
// and its hash from those children's (ID, hash) pairs — so a file becomes
// pending again if a child's summary changes OR a child is added/removed.
func pendingFiles(g *graph.Graph, s *store.Store, limit int) ([]PendingSummary, error) {
	childrenByFile := make(map[string][]*graph.Node)
	for _, n := range g.Nodes() {
		// hasOwnSourceSpan, not summarizableTypes: file/package aggregation
		// is specifically about Go source files containing Struct/
		// Interface/Function declarations. A migration-sourced Table node
		// also carries a File (the .sql file it came from) but must not be
		// swept into this aggregation — it already gets its own node-level
		// summary (see sourceTextFor), and a .sql file isn't a Go file
		// belonging to a package the way pendingModules expects below.
		if !hasOwnSourceSpan(n.Type) || n.File == "" {
			continue
		}
		childrenByFile[n.File] = append(childrenByFile[n.File], n)
	}

	nodeSummaries, err := s.SummariesByLevel(LevelNode)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(childrenByFile))
	for f := range childrenByFile {
		files = append(files, f)
	}
	sort.Strings(files)

	var out []PendingSummary
	for _, file := range files {
		children := childrenByFile[file]
		sort.Slice(children, func(i, j int) bool { return children[i].ID < children[j].ID })

		var parts []string
		var hashParts []string
		complete := true
		for _, c := range children {
			rec, ok := nodeSummaries[c.ID]
			if !ok || rec.Hash != c.Hash {
				complete = false
				break
			}
			parts = append(parts, fmt.Sprintf("- %s: %s", c.ID, rec.Summary))
			hashParts = append(hashParts, c.ID+":"+rec.Hash)
		}
		if !complete {
			continue // not all children summarized yet — file isn't eligible
		}

		fileHash := graph.ContentHash(strings.Join(hashParts, "|"))
		existingHash, ok, err := s.SummaryHash(file, LevelFile)
		if err != nil {
			return nil, err
		}
		if ok && existingHash == fileHash {
			continue
		}

		out = append(out, PendingSummary{
			ID: file, Type: "File", File: file, Hash: fileHash,
			Source: strings.Join(parts, "\n"),
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// pendingModules derives a package's content from its files' summaries,
// mirroring pendingFiles one level up.
func pendingModules(g *graph.Graph, s *store.Store, limit int) ([]PendingSummary, error) {
	filesByPkg := make(map[string]map[string]bool)
	for _, n := range g.Nodes() {
		// hasOwnSourceSpan, not summarizableTypes — see pendingFiles' doc
		// comment on the same check.
		if !hasOwnSourceSpan(n.Type) || n.File == "" {
			continue
		}
		pkgPath, _ := n.Properties["package"].(string)
		if pkgPath == "" {
			continue
		}
		if filesByPkg[pkgPath] == nil {
			filesByPkg[pkgPath] = make(map[string]bool)
		}
		filesByPkg[pkgPath][n.File] = true
	}

	fileSummaries, err := s.SummariesByLevel(LevelFile)
	if err != nil {
		return nil, err
	}

	pkgs := make([]string, 0, len(filesByPkg))
	for p := range filesByPkg {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)

	var out []PendingSummary
	for _, pkgPath := range pkgs {
		files := make([]string, 0, len(filesByPkg[pkgPath]))
		for f := range filesByPkg[pkgPath] {
			files = append(files, f)
		}
		sort.Strings(files)

		var parts, hashParts []string
		complete := true
		for _, f := range files {
			rec, ok := fileSummaries[f]
			if !ok {
				complete = false
				break
			}
			parts = append(parts, fmt.Sprintf("- %s: %s", f, rec.Summary))
			hashParts = append(hashParts, f+":"+rec.Hash)
		}
		if !complete {
			continue
		}

		moduleID := graph.PackageID(pkgPath)
		moduleHash := graph.ContentHash(strings.Join(hashParts, "|"))
		existingHash, ok, err := s.SummaryHash(moduleID, LevelModule)
		if err != nil {
			return nil, err
		}
		if ok && existingHash == moduleHash {
			continue
		}

		out = append(out, PendingSummary{
			ID: moduleID, Type: "Package", Hash: moduleHash,
			Source: strings.Join(parts, "\n"),
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// readSourceLines returns lines [start, end] (1-indexed, inclusive) of
// file. It re-reads from disk rather than storing source in the database,
// so `summarize pending` must run against a repo still at (approximately)
// the state it was built from.
func readSourceLines(file string, start, end int) (string, error) {
	if file == "" || start == 0 {
		return "", fmt.Errorf("summarizer: no source location for file %q", file)
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if lineNo > end {
			break
		}
		if lineNo >= start {
			lines = append(lines, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}
