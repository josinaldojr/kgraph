# node-search

## Purpose

TBD - extracted from kgraph-mvp change. Provides lexical, term-overlap node search (`SearchNodes`) over node summaries (falling back to signature/name) with no external embedding API or vector database.

## Requirements

### Requirement: Lexical node search
The system SHALL implement `SearchNodes(query string, topK int) []Node` that scores each node by term overlap between the (lowercased, tokenized) query and the node's summary — falling back to its signature/name when it has no summary yet — and returns the `topK` highest-scoring nodes in descending score order, with no external embedding API or vector database involved.

#### Scenario: Ranked results for a topic query
- **WHEN** `SearchNodes` is called with a natural-language query and `topK` greater than zero
- **THEN** the returned nodes are ordered by descending term-overlap score and number at most `topK`

#### Scenario: Unsummarized node still matches on name/signature
- **WHEN** a node has no stored summary yet
- **THEN** `SearchNodes` still considers it for matching, scoring against its signature and name instead of a summary
