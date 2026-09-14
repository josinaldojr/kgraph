package build

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/josinaldojr/kgraph/internal/gitutil"
	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/parser"
	"github.com/josinaldojr/kgraph/internal/parser/common"
	"github.com/josinaldojr/kgraph/internal/store"
	"github.com/josinaldojr/kgraph/internal/summarizer"
)

// UpdateResult reports what an incremental Update actually reprocessed.
type UpdateResult struct {
	FromCommit   string
	ToCommit     string
	ChangedFiles []string
	Stats        store.SaveStats
	StaleMarked  int
	NoChange     bool // true if HEAD hadn't moved since the last build
	Warnings     []string
}

// Update runs incremental reprocessing: it diffs the repo at repoPath
// against the commit recorded by the last build/update, re-extracts only
// the Go packages containing changed files (plus SQL migrations, if any
// changed), and marks the direct neighbors of every changed/removed node
// stale so they resurface in the next `summarize pending` — per
// design.md's Decision 9 and the incremental-update spec. After
// re-extraction, the enrich and analyze stages re-run over the full
// (reloaded) graph — see Options — so rationale, confidence, degree,
// god-node, and community metadata stay correct after the change.
func Update(repoPath, dbPath string, opts ...Options) (UpdateResult, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	if !gitutil.IsRepo(repoPath) {
		return UpdateResult{}, fmt.Errorf("update: %s is not a git repository", repoPath)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("update: opening store at %s: %w", dbPath, err)
	}
	defer s.Close()

	fromCommit, ok, err := s.LastCommit(repoPath)
	if err != nil {
		return UpdateResult{}, err
	}
	if !ok {
		return UpdateResult{}, fmt.Errorf("update: no prior build found for %s — run `kgraph build` first", repoPath)
	}

	toCommit, err := gitutil.CurrentCommit(repoPath)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("update: resolving current commit: %w", err)
	}
	if toCommit == fromCommit {
		return UpdateResult{FromCommit: fromCommit, ToCommit: toCommit, NoChange: true}, nil
	}

	changedRel, err := gitutil.ChangedFiles(repoPath, fromCommit)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("update: diffing %s..%s: %w", fromCommit, toCommit, err)
	}
	if len(changedRel) == 0 {
		if err := s.SetLastCommit(repoPath, toCommit); err != nil {
			return UpdateResult{}, err
		}
		return UpdateResult{FromCommit: fromCommit, ToCommit: toCommit, NoChange: true}, nil
	}

	old, err := s.LoadGraph()
	if err != nil {
		return UpdateResult{}, fmt.Errorf("update: loading prior graph: %w", err)
	}

	extToLang := extensionLanguageMap()

	var (
		changedAbs []string
		hasSQL     bool
		// patternsByLang groups changed directories by language, so a
		// mixed-language changeset doesn't hand every extractor every
		// other language's directories too.
		patternsByLang = map[common.Language]map[string]bool{}
	)
	for _, rel := range changedRel {
		abs, err := filepath.Abs(filepath.Join(repoPath, rel))
		if err != nil {
			continue
		}
		changedAbs = append(changedAbs, abs)
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".sql" {
			hasSQL = true
			continue
		}
		lang, ok := extToLang[ext]
		if !ok {
			continue
		}
		if patternsByLang[lang] == nil {
			patternsByLang[lang] = map[string]bool{}
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "." {
			patternsByLang[lang]["."] = true
		} else {
			patternsByLang[lang]["./"+dir] = true
		}
	}

	warnings := []string{}
	var newSub *graph.Graph
	if len(patternsByLang) > 0 {
		patternLists := make(map[common.Language][]string, len(patternsByLang))
		for lang, set := range patternsByLang {
			list := make([]string, 0, len(set))
			for p := range set {
				list = append(list, p)
			}
			patternLists[lang] = list
		}
		g, w, err := parser.ExtractPackages(repoPath, patternLists, old)
		if err != nil {
			return UpdateResult{}, fmt.Errorf("update: re-extracting %v: %w", patternLists, err)
		}
		newSub = g
		warnings = append(warnings, w...)
	} else {
		newSub = graph.New()
	}

	if hasSQL {
		w, err := parser.ExtractMigrations(repoPath, newSub)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("migrations: %v", err))
		} else {
			warnings = append(warnings, w...)
		}
	}

	staleTargets := invalidationTargets(old, newSub, changedAbs)

	for _, abs := range changedAbs {
		if err := s.DeleteNodesForFile(abs); err != nil {
			return UpdateResult{}, fmt.Errorf("update: clearing stale nodes for %s: %w", abs, err)
		}
	}

	stats, err := s.SaveGraph(newSub)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("update: saving reprocessed graph: %w", err)
	}

	full, err := s.LoadGraph()
	if err != nil {
		return UpdateResult{}, fmt.Errorf("update: reloading full graph for enrich/analyze: %w", err)
	}
	enrichWarnings, err := enrichAndAnalyze(full, opt)
	warnings = append(warnings, enrichWarnings...)
	if err != nil {
		return UpdateResult{}, err
	}
	if _, err := s.SaveGraph(full); err != nil {
		return UpdateResult{}, fmt.Errorf("update: saving enriched/analyzed graph: %w", err)
	}
	if err := s.RefreshNodeProperties(full); err != nil {
		return UpdateResult{}, fmt.Errorf("update: persisting analytics metadata: %w", err)
	}

	staleMarked := 0
	for id := range staleTargets {
		// Only mark nodes that still exist somewhere (old graph, since a
		// removed node has nothing to mark) — the store no-ops harmlessly
		// either way, this just keeps the reported count meaningful.
		if old.Node(id) != nil {
			if err := s.MarkSummaryStale(id, summarizer.LevelNode); err != nil {
				return UpdateResult{}, err
			}
			staleMarked++
		}
	}

	if err := s.SetLastCommit(repoPath, toCommit); err != nil {
		return UpdateResult{}, err
	}

	return UpdateResult{
		FromCommit: fromCommit, ToCommit: toCommit, ChangedFiles: changedRel,
		Stats: stats, StaleMarked: staleMarked, Warnings: warnings,
	}, nil
}

// extensionLanguageMap builds a file-extension → Language lookup from every
// extractor registered in the default factory (via FileExtensions()),
// rather than hardcoding the extension list here — a newly registered
// extractor (parser.RegisterExtractor) is picked up automatically.
func extensionLanguageMap() map[string]common.Language {
	out := make(map[string]common.Language)
	for _, lang := range parser.Factory().Languages() {
		ext, err := parser.Factory().Get(lang)
		if err != nil {
			continue
		}
		for _, e := range ext.FileExtensions() {
			out[strings.ToLower(e)] = lang
		}
	}
	return out
}

// invalidationTargets returns the IDs of every changed/removed node's
// direct neighbors (from the OLD graph, before those edges are deleted),
// per the "direct-neighbor summary invalidation" requirement. A node
// counts as changed if it's new or its hash differs from the old graph's;
// removed nodes (present in old, absent from newSub, within a changed
// file) contribute their old neighbors too.
func invalidationTargets(old, newSub *graph.Graph, changedFiles []string) map[string]bool {
	changedSet := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changedSet[f] = true
	}

	targets := make(map[string]bool)
	addNeighbors := func(id string) {
		targets[id] = true
		for _, e := range old.OutEdges(id) {
			targets[e.DstID] = true
		}
		for _, e := range old.InEdges(id) {
			targets[e.SrcID] = true
		}
	}

	for _, n := range newSub.Nodes() {
		oldNode := old.Node(n.ID)
		if oldNode == nil || oldNode.Hash != n.Hash {
			addNeighbors(n.ID)
		}
	}
	for _, n := range old.Nodes() {
		if n.File != "" && changedSet[n.File] && newSub.Node(n.ID) == nil {
			addNeighbors(n.ID)
		}
	}
	return targets
}
