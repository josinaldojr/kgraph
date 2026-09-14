// Package goparser implements the common.Extractor interface for Go source
// code. It wraps the existing Go extraction logic (go/ast, go/types,
// golang.org/x/tools/go/packages) behind the multi-language interface.
package goparser

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/packages"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/parser/common"
)

// GoExtractor implements common.Extractor for Go source code.
type GoExtractor struct{}

// NewGoExtractor creates a new GoExtractor.
func NewGoExtractor() *GoExtractor {
	return &GoExtractor{}
}

// Language returns the language this extractor handles.
func (e *GoExtractor) Language() common.Language {
	return common.LangGo
}

// FileExtensions returns the file extensions this extractor processes.
func (e *GoExtractor) FileExtensions() []string {
	return []string{".go"}
}

// ExtractRepo extracts the full graph from a Go repository at repoPath.
func (e *GoExtractor) ExtractRepo(repoPath string) (*graph.Graph, []string, error) {
	g, warnings, err := e.ExtractPackages(repoPath, []string{"./..."}, nil)
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

// ExtractPackages extracts a scoped subset of Go packages.
func (e *GoExtractor) ExtractPackages(repoPath string, patterns []string, knownInternal map[string]bool) (*graph.Graph, []string, error) {
	cfg := &packages.Config{
		Dir: repoPath,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, nil, fmt.Errorf("go parser: loading packages %v under %s: %w", patterns, repoPath, err)
	}
	if len(pkgs) == 0 {
		return nil, nil, fmt.Errorf("go parser: no Go packages found for %v under %s", patterns, repoPath)
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
	ex.extractTypes()
	ex.mapORMFields()
	ex.extractFuncs()
	ex.extractCalls()

	return ex.g, ex.warnings, nil
}

// extractor holds the state shared across extraction passes for a single
// ExtractRepo call.
type extractor struct {
	g         *graph.Graph
	fset      *token.FileSet
	pkgs      []*packages.Package
	pkgByPath map[string]*packages.Package
	internal  map[string]bool
	ormFields []ormFieldCandidate
	warnings  []string
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
