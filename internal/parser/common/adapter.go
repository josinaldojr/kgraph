// Package common defines the shared interfaces and types for multi-language
// code extraction. All language-specific parsers implement the Extractor
// interface, and the ExtractorFactory routes to the correct parser based on
// detected language.
package common

import "kgraph/internal/graph"

// Language identifies a programming language supported by kgraph.
type Language string

const (
	LangGo         Language = "go"
	LangJava       Language = "java"
	LangTypeScript Language = "typescript"
	LangJavaScript Language = "javascript"
	LangPython     Language = "python"
)

// DetectionResult holds the outcome of language detection for a repository.
type DetectionResult struct {
	// Primary is the main language detected (first match by priority).
	Primary Language
	// All contains all detected languages (for multi-language projects).
	All []Language
	// Markers maps each detected language to the marker files that triggered it.
	Markers map[Language][]string
}

// Extractor is the interface that all language-specific parsers implement.
// Each extractor knows how to parse source code in its language and produce
// a graph of code entities and relationships.
type Extractor interface {
	// ExtractRepo extracts the full graph from a repository at repoPath.
	// Returns the graph, any non-fatal warnings, and an error if extraction
	// failed entirely.
	ExtractRepo(repoPath string) (*graph.Graph, []string, error)

	// ExtractPackages extracts a scoped subset of the repository, used by
	// incremental updates. patterns are specific directories/files;
	// knownInternal are packages already known from a prior build.
	ExtractPackages(repoPath string, patterns []string, knownInternal map[string]bool) (*graph.Graph, []string, error)

	// FileExtensions returns the file extensions this extractor processes
	// (e.g., [".go"] for Go, [".java"] for Java).
	FileExtensions() []string

	// Language returns the language this extractor handles.
	Language() Language
}
