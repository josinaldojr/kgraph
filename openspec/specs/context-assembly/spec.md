# context-assembly

## Purpose

TBD - extracted from kgraph-mvp change. Assembles a token-budgeted context view (`GetContext`) around a resolved target node, expanding its subgraph using node summaries and direct relations rather than raw source code.

## Requirements

### Requirement: Target resolution
`GetContext` SHALL resolve its `target` argument by exact match against known file paths, struct names, or function names first, falling back to `SearchNodes` semantic search only when no exact match exists.

#### Scenario: Exact match short-circuits search
- **WHEN** `target` exactly matches an existing file path, struct name, or function name
- **THEN** the system resolves that node directly without invoking semantic search

#### Scenario: Fallback to semantic search
- **WHEN** `target` does not exactly match any known file, struct, or function name
- **THEN** the system resolves the target via `SearchNodes` and uses the top result

### Requirement: Summary-based subgraph expansion
`GetContext` SHALL expand the subgraph around the resolved target up to `hops` edges out, and SHALL render the result using node summaries and direct relations (signature, callers, callees, tables read/written) rather than raw source code.

#### Scenario: Two-hop expansion excludes source
- **WHEN** `GetContext` is called with `hops=2`
- **THEN** the rendered context includes nodes up to two edges away from the target, described only by their summaries and relations, with no raw source code included

### Requirement: Token-budgeted output
`GetContext` SHALL respect the `maxTokens` limit, prioritizing the target node's own summary and direct (one-hop) relations, and truncating farther/lower-priority nodes first when the estimated token count would exceed the budget.

#### Scenario: Truncation under budget pressure
- **WHEN** the full expanded subgraph's estimated token count exceeds `maxTokens`
- **THEN** the system omits farther-hop nodes first while still including the target's own summary and its direct relations, and the rendered output's estimated token count does not exceed `maxTokens`
