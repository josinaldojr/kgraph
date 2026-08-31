// Package parser extracts a code knowledge graph from a Go repository using
// the standard library (go/ast, go/parser) plus golang.org/x/tools/go/packages
// for best-effort call-edge resolution. See design.md's "Decisions" #1 for
// why this repo uses go/packages instead of tree-sitter.
package parser

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/packages"

	"kgraph/internal/graph"
)

// extractor holds the state shared across extraction passes for a single
// ExtractRepo call.
type extractor struct {
	g         *graph.Graph
	fset      *token.FileSet
	pkgs      []*packages.Package
	pkgByPath map[string]*packages.Package
	internal  map[string]bool // PkgPath -> true for packages within the loaded module
	ormFields []ormFieldCandidate
	warnings  []string
}

// ExtractRepo parses every Go package under repoPath, extracts packages,
// structs, interfaces, functions, fields, ORM-mapped tables/columns, SQL
// migrations, and call/has_field/has_method/imports edges, and returns the
// resulting graph. Parsing/type errors in individual files or packages are
// collected as non-fatal warnings (returned alongside the graph) rather than
// aborting extraction — call-edge and type resolution are best-effort, per
// design.md.
func ExtractRepo(repoPath string) (*graph.Graph, []string, error) {
	g, warnings, err := ExtractPackages(repoPath, []string{"./..."}, nil)
	if err != nil {
		return nil, nil, err
	}
	migWarnings, err := ExtractMigrations(repoPath, g)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("migrations: %v", err))
	} else {
		warnings = append(warnings, migWarnings...)
	}
	return g, warnings, nil
}

// ExtractPackages is ExtractRepo scoped to a specific set of go/packages
// patterns (e.g. "./internal/foo" for just that directory's package,
// non-recursively) instead of the whole module — used by incremental
// update to reprocess only the packages containing changed files. Unlike
// ExtractRepo, it does NOT also run SQL migration extraction (that's a
// repo-wide file walk, orthogonal to which Go packages changed) — callers
// that need migrations re-scanned call ExtractMigrations themselves.
//
// knownInternal seeds the internal/external import classification with
// import paths already known (from a prior full build) to belong to the
// module, so a scoped load — which by itself only "sees" the packages it
// was asked to load — doesn't mistake an unloaded-but-still-internal
// sibling package for an ExternalDependency. ExtractRepo passes nil since
// a "./..." load already sees the whole module.
func ExtractPackages(repoPath string, patterns []string, knownInternal map[string]bool) (*graph.Graph, []string, error) {
	cfg := &packages.Config{
		Dir: repoPath,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, nil, fmt.Errorf("parser: loading packages %v under %s: %w", patterns, repoPath, err)
	}
	if len(pkgs) == 0 {
		return nil, nil, fmt.Errorf("parser: no Go packages found for %v under %s", patterns, repoPath)
	}

	internal := make(map[string]bool, len(pkgs)+len(knownInternal))
	for path, v := range knownInternal {
		if v {
			internal[path] = true
		}
	}

	ex := &extractor{
		g:         graph.New(),
		pkgs:      pkgs,
		pkgByPath: make(map[string]*packages.Package, len(pkgs)),
		internal:  internal,
	}
	for _, p := range pkgs {
		if len(p.Syntax) > 0 {
			ex.fset = p.Fset
		}
		ex.pkgByPath[p.PkgPath] = p
		ex.internal[p.PkgPath] = true
		for _, e := range p.Errors {
			ex.warnings = append(ex.warnings, fmt.Sprintf("%s: %s", p.PkgPath, e.Msg))
		}
	}

	ex.extractPackagesAndImports()
	ex.extractTypes() // structs, interfaces, fields -> pass 2
	ex.mapORMFields() // struct tags -> Table/Column nodes -> pass 2b
	ex.extractFuncs() // functions/methods + has_method -> pass 3a
	ex.extractCalls() // calls edges -> pass 3b

	return ex.g, ex.warnings, nil
}

// forEachFile invokes fn for every parsed syntax file across the module's
// loaded packages.
func (ex *extractor) forEachFile(fn func(pkg *packages.Package, file *ast.File)) {
	for _, pkg := range ex.pkgs {
		for _, file := range pkg.Syntax {
			fn(pkg, file)
		}
	}
}
