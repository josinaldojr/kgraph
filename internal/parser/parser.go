// Package parser provides the multi-language code extraction entry point.
// It uses a factory pattern to route to the correct language-specific
// extractor based on detected language.
package parser

import (
	"fmt"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/parser/common"
	goparser "github.com/josinaldojr/kgraph/internal/parser/go"
	javaparser "github.com/josinaldojr/kgraph/internal/parser/java"
	pythonparser "github.com/josinaldojr/kgraph/internal/parser/python"
	typescriptparser "github.com/josinaldojr/kgraph/internal/parser/typescript"
)

// defaultFactory is the package-level factory with all built-in extractors
// registered.
var defaultFactory *common.ExtractorFactory

func init() {
	defaultFactory = common.NewExtractorFactory()
	defaultFactory.Register(common.LangGo, goparser.NewGoExtractor())
	defaultFactory.Register(common.LangJava, javaparser.NewJavaExtractor())
	defaultFactory.Register(common.LangTypeScript, typescriptparser.NewTypeScriptExtractor())
	defaultFactory.Register(common.LangJavaScript, typescriptparser.NewJavaScriptExtractor())
	defaultFactory.Register(common.LangPython, pythonparser.NewPythonExtractor())
}

// ExtractRepo detects the language(s) of the repository at repoPath and
// extracts the full code knowledge graph. For multi-language projects,
// graphs from each language are merged.
func ExtractRepo(repoPath string) (*graph.Graph, []string, error) {
	detector := common.NewLanguageDetector()
	result, err := detector.Detect(repoPath)
	if err != nil {
		return nil, nil, fmt.Errorf("parser: detecting language: %w", err)
	}

	extractors, factoryWarnings := defaultFactory.ForDetection(result)
	if len(extractors) == 0 {
		return nil, factoryWarnings, fmt.Errorf("parser: no extractors available for detected languages %v", result.All)
	}

	var graphs []*graph.Graph
	var allWarnings []string
	allWarnings = append(allWarnings, factoryWarnings...)

	for _, ext := range extractors {
		g, warnings, err := ext.ExtractRepo(repoPath)
		if err != nil {
			allWarnings = append(allWarnings, fmt.Sprintf("%s: %v", ext.Language(), err))
			continue
		}
		allWarnings = append(allWarnings, warnings...)
		allWarnings = append(allWarnings, g.IDConflicts()...)
		graphs = append(graphs, g)
	}

	if len(graphs) == 0 {
		return nil, allWarnings, fmt.Errorf("parser: all extractors failed")
	}

	merged, mergeWarnings := common.MergeGraphsWithWarnings(graphs...)
	allWarnings = append(allWarnings, mergeWarnings...)

	return merged, allWarnings, nil
}

// ExtractPackages extracts a scoped subset of the repository, used by
// incremental updates. patternsByLang is the set of changed directories per
// language (a caller with a mixed-language changeset must not hand every
// extractor every language's directories); oldGraph is the previously
// persisted graph (possibly multi-language), from which each extractor's
// knownInternal is scoped to just its own language via
// common.KnownInternalForLanguage.
func ExtractPackages(repoPath string, patternsByLang map[common.Language][]string, oldGraph *graph.Graph) (*graph.Graph, []string, error) {
	detector := common.NewLanguageDetector()
	result, err := detector.Detect(repoPath)
	if err != nil {
		return nil, nil, fmt.Errorf("parser: detecting language: %w", err)
	}

	extractors, factoryWarnings := defaultFactory.ForDetection(result)
	if len(extractors) == 0 {
		return nil, factoryWarnings, fmt.Errorf("parser: no extractors available for detected languages %v", result.All)
	}

	var graphs []*graph.Graph
	var allWarnings []string
	allWarnings = append(allWarnings, factoryWarnings...)

	for _, ext := range extractors {
		langPatterns := patternsByLang[ext.Language()]
		if len(langPatterns) == 0 {
			// Detected in the repo, but nothing of this language changed
			// this round — skip it rather than call ExtractPackages with
			// an empty pattern list, which some extractors (Go, via
			// packages.Load with zero patterns) treat as an error.
			continue
		}
		known := common.KnownInternalForLanguage(oldGraph, ext.Language())
		g, warnings, err := ext.ExtractPackages(repoPath, langPatterns, known)
		if err != nil {
			allWarnings = append(allWarnings, fmt.Sprintf("%s: %v", ext.Language(), err))
			continue
		}
		allWarnings = append(allWarnings, warnings...)
		allWarnings = append(allWarnings, g.IDConflicts()...)
		graphs = append(graphs, g)
	}

	if len(graphs) == 0 {
		return nil, allWarnings, fmt.Errorf("parser: all extractors failed")
	}

	merged, mergeWarnings := common.MergeGraphsWithWarnings(graphs...)
	allWarnings = append(allWarnings, mergeWarnings...)

	return merged, allWarnings, nil
}

// RegisterExtractor registers a custom extractor for a language.
// This allows extending kgraph with additional language support.
func RegisterExtractor(lang common.Language, ext common.Extractor) {
	defaultFactory.Register(lang, ext)
}

// Detector returns the language detector for external use.
func Detector() *common.LanguageDetector {
	return common.NewLanguageDetector()
}

// Factory returns the default extractor factory for external use.
func Factory() *common.ExtractorFactory {
	return defaultFactory
}

// ExtractMigrations parses SQL migration files and adds them to the graph.
// This is re-exported from the Go parser for backward compatibility.
func ExtractMigrations(repoPath string, g *graph.Graph) ([]string, error) {
	return goparser.ExtractMigrations(repoPath, g)
}
