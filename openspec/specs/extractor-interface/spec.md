# extractor-interface

## Purpose

Defines the common `Extractor` interface that every per-language parser implements, and the `ExtractorFactory` that routes to the correct extractor(s) based on detected language(s), so the build and update pipelines are language-agnostic.

## Requirements

### Requirement: Extractor interface SHALL be implemented by every language parser
Every language parser SHALL implement the `Extractor` interface: `ExtractRepo(repoPath string) (*graph.Graph, []string, error)` for full-repository extraction, `ExtractPackages(repoPath string, patterns []string, knownInternal map[string]bool) (*graph.Graph, []string, error)` for scoped extraction used by incremental updates, `FileExtensions() []string` reporting the file extensions it handles, and `Language() Language` reporting the language it handles.

#### Scenario: Full-repository extraction
- **WHEN** `ExtractRepo` is called with a repository path
- **THEN** the extractor SHALL return the complete graph for that language's portion of the repository, any collected warnings, and an error only on unrecoverable failure

#### Scenario: Scoped extraction with empty patterns
- **WHEN** `ExtractPackages` is called with an empty `patterns` slice
- **THEN** the extractor SHALL return an empty graph rather than an error

### Requirement: ExtractorFactory SHALL register and route extractors
The system SHALL provide an `ExtractorFactory` that maintains a registry of extractors keyed by `Language`, supports registering an extractor via `Register(lang Language, ext Extractor)`, retrieving one via `Get(lang Language) (Extractor, bool)`, and resolving the applicable set of extractors for a language-detection result via `ForDetection(result DetectionResult) []Extractor`.

#### Scenario: Get returns registered extractor
- **WHEN** `Get` is called with a `Language` that has a registered extractor
- **THEN** it SHALL return that extractor and `true`

#### Scenario: Get returns false for unregistered language
- **WHEN** `Get` is called with a `Language` that has no registered extractor
- **THEN** it SHALL return `false` and a nil extractor

#### Scenario: ForDetection returns all matched extractors for multi-language repos
- **WHEN** `ForDetection` is called with a `DetectionResult` whose `All` field lists multiple languages
- **THEN** it SHALL return one extractor per detected language that has a registered extractor

### Requirement: Build pipeline SHALL use the factory instead of calling a single parser directly
The build pipeline SHALL detect the repository's language(s) via `LanguageDetector.Detect`, obtain the applicable extractors via `ExtractorFactory.ForDetection`, call `ExtractRepo` on each, and merge the resulting graphs (per the multi-language-merge capability) rather than calling the Go parser directly.

#### Scenario: Single-language repo uses one extractor
- **WHEN** a repository's language detection yields a single language
- **THEN** the build pipeline invokes exactly that language's extractor and does not attempt to merge multiple graphs

#### Scenario: Multi-language repo uses multiple extractors
- **WHEN** a repository's language detection yields more than one language
- **THEN** the build pipeline invokes each matched extractor's `ExtractRepo` and merges all resulting graphs into one

### Requirement: Update pipeline SHALL route changed files to extractors by extension
The update pipeline SHALL determine the languages present in the existing graph, route each changed file to the extractor matching its file extension, invoke that extractor's `ExtractPackages` with scoped patterns, and merge the resulting subgraphs.

#### Scenario: Update loads existing languages from the graph
- **WHEN** an update runs against a repository with a previously built graph
- **THEN** the pipeline determines which languages are present from the graph's node properties rather than re-running full language detection

### Requirement: Extractor failures SHALL be isolated (best-effort)
If one extractor fails while processing a multi-language repository, the failure SHALL NOT abort extraction for the other languages; the error SHALL be collected as a warning and a partial graph SHALL be returned.

#### Scenario: One extractor errors, others succeed
- **WHEN** one of several matched extractors returns an error from `ExtractRepo`
- **THEN** the build pipeline continues running the remaining extractors, includes their results in the merged graph, and surfaces the failed extractor's error as a warning rather than aborting the build

### Requirement: Unsupported language SHALL produce a clear error
The `ExtractorFactory` SHALL return a clear, actionable error when asked to resolve a language for which no extractor is registered and no fallback applies.

#### Scenario: Detection yields an unregistered language
- **WHEN** language detection identifies a language with no corresponding registered extractor
- **THEN** the factory SHALL surface a clear error identifying the unsupported language rather than silently skipping it
