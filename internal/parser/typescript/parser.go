// Package typescriptparser implements the common.Extractor interface for
// TypeScript and JavaScript source code.
package typescriptparser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"kgraph/internal/graph"
	"kgraph/internal/parser/common"
)

// TypeScriptExtractor implements common.Extractor for TypeScript/JavaScript source code.
type TypeScriptExtractor struct {
	isJavaScript bool
}

// NewTypeScriptExtractor creates a new TypeScriptExtractor.
func NewTypeScriptExtractor() *TypeScriptExtractor {
	return &TypeScriptExtractor{isJavaScript: false}
}

// NewJavaScriptExtractor creates a new JavaScriptExtractor.
func NewJavaScriptExtractor() *TypeScriptExtractor {
	return &TypeScriptExtractor{isJavaScript: true}
}

// Language returns the language this extractor handles.
func (e *TypeScriptExtractor) Language() common.Language {
	if e.isJavaScript {
		return common.LangJavaScript
	}
	return common.LangTypeScript
}

// FileExtensions returns the file extensions this extractor processes.
func (e *TypeScriptExtractor) FileExtensions() []string {
	if e.isJavaScript {
		return []string{".js", ".jsx"}
	}
	return []string{".ts", ".tsx"}
}

// pendingEdge is an edge whose destination node may be defined in a file
// this extractor hasn't visited yet (extends/implements/injected targets,
// or a best-effort call target). Resolved after the whole repo is walked.
type pendingEdge struct {
	edgeType graph.EdgeType
	srcID    string
	dstID    string
}

// ExtractRepo extracts the full graph from a TypeScript/JavaScript repository at repoPath.
func (e *TypeScriptExtractor) ExtractRepo(repoPath string) (*graph.Graph, []string, error) {
	g := graph.New()
	warnings := e.extractDependencies(repoPath, g)

	walkWarnings, err := e.extractWalk(g, repoPath, nil, func(string) bool { return true })
	warnings = append(warnings, walkWarnings...)
	if err != nil {
		return nil, warnings, err
	}

	return g, warnings, nil
}

// ExtractPackages extracts a scoped subset of TypeScript/JavaScript
// packages: only files under the given patterns (directories, relative to
// repoPath, e.g. "." or "./src/services") are walked. An empty patterns
// slice yields an empty graph. knownInternal (module import paths already
// known from a prior build) augments the internal-module set used to
// classify import targets as internal vs external, since a scoped walk
// alone cannot discover modules outside patterns.
func (e *TypeScriptExtractor) ExtractPackages(repoPath string, patterns []string, knownInternal map[string]bool) (*graph.Graph, []string, error) {
	if len(patterns) == 0 {
		return graph.New(), nil, nil
	}

	allowed := make(map[string]bool, len(patterns))
	for _, p := range patterns {
		allowed[filepath.ToSlash(p)] = true
	}

	g := graph.New()
	warnings := e.extractDependencies(repoPath, g)

	walkWarnings, err := e.extractWalk(g, repoPath, knownInternal, func(relDir string) bool {
		return allowed[relDir]
	})
	warnings = append(warnings, walkWarnings...)
	if err != nil {
		return nil, warnings, err
	}

	return g, warnings, nil
}

// extractWalk walks repoPath, running extractFile on every file matching
// FileExtensions whose directory (relative to repoPath, slash-normalized)
// include accepts, then resolves cross-file references — pending
// extends/implements/injected/calls edges, then import edges — against
// the resulting graph. Deferring both until after the walk means
// resolution doesn't depend on filesystem walk order and can classify a
// reference as internal or external using the complete internal-module
// set this pass produces (merged with knownInternal, for a scoped walk
// that can't see modules outside its own patterns).
func (e *TypeScriptExtractor) extractWalk(g *graph.Graph, repoPath string, knownInternal map[string]bool, include func(relDir string) bool) ([]string, error) {
	var warnings []string
	var pending []pendingEdge
	var pendingImports []pendingImport

	extensions := e.FileExtensions()

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		isMatch := false
		for _, e := range extensions {
			if ext == e {
				isMatch = true
				break
			}
		}
		if !isMatch {
			return nil
		}

		rel, relErr := filepath.Rel(repoPath, path)
		if relErr != nil {
			return nil
		}
		relDir := filepath.ToSlash(filepath.Dir(rel))
		if relDir != "." {
			relDir = "./" + relDir
		}
		if !include(relDir) {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", path, readErr))
			return nil
		}

		fileWarnings, filePending, fileImports := e.extractFile(g, path, string(content), repoPath)
		warnings = append(warnings, fileWarnings...)
		pending = append(pending, filePending...)
		pendingImports = append(pendingImports, fileImports...)
		return nil
	})
	if err != nil {
		return warnings, fmt.Errorf("typescript parser: walking %s: %w", repoPath, err)
	}

	warnings = append(warnings, resolvePendingEdges(g, pending)...)

	internal := make(map[string]bool, len(knownInternal))
	for path, v := range knownInternal {
		if v {
			internal[path] = true
		}
	}
	for _, n := range g.Nodes() {
		if n.Type == graph.NodeTypePackage {
			internal[strings.TrimPrefix(n.ID, "pkg:")] = true
		}
	}
	warnings = append(warnings, resolveImports(g, pendingImports, internal, e.Language())...)

	return warnings, nil
}

// resolvePendingEdges adds every pending edge whose destination node now
// exists in the graph. A missing destination (an external type, or an
// unresolved call/injection target) is expected and skipped silently —
// call/reference resolution here is best-effort, without type information.
func resolvePendingEdges(g *graph.Graph, pending []pendingEdge) []string {
	var warnings []string
	for _, pe := range pending {
		if g.Node(pe.dstID) == nil {
			continue
		}
		if err := g.AddEdge(&graph.Edge{
			ID:    graph.EdgeID(pe.edgeType, pe.srcID, pe.dstID),
			Type:  pe.edgeType,
			SrcID: pe.srcID,
			DstID: pe.dstID,
		}); err != nil {
			warnings = append(warnings, err.Error())
		}
	}
	return warnings
}

// pendingImport is an `import`/`require` statement's target, deferred
// (like pendingEdge) until the whole walk completes and the
// internal-module set is known, since classifying it as internal vs
// external can't be decided file-by-file. target is already resolved
// (relative specifiers are resolved to the internal dotted-module scheme
// via resolveImportModule; a non-relative specifier, e.g. an npm package
// name, is left as-is).
type pendingImport struct {
	srcID  string
	target string
}

// resolveImports creates an `imports` edge for every pending import,
// classifying its target as internal (present in internal) or external —
// mirroring the Go extractor's own internal/external classification for
// import edges. An internal target gets an edge to that Package node. An
// external target gets an edge to an ExternalDependency node: one a
// package.json parse already created is reused when the import specifier's
// root (e.g. "express" from "express/lib/foo", or "@nestjs/common" from a
// scoped package) matches a declared dependency name, otherwise a new node
// (keyed by that root) is created on the fly.
func resolveImports(g *graph.Graph, imports []pendingImport, internal map[string]bool, lang common.Language) []string {
	var warnings []string
	for _, pi := range imports {
		if pi.target == "" {
			continue
		}

		var dstID string
		if internal[pi.target] {
			dstID = graph.PackageID(pi.target)
		} else {
			root := tsDependencyRoot(pi.target)
			dstID = graph.ExternalDependencyID(root)
			if g.Node(dstID) == nil {
				g.AddNode(&graph.Node{
					ID:   dstID,
					Type: graph.NodeTypeExternalDependency,
					Properties: map[string]any{
						"name":     root,
						"language": string(lang),
					},
					Hash: graph.ContentHash(root),
				})
			}
		}

		if err := g.AddEdge(&graph.Edge{
			ID:    graph.EdgeID(graph.EdgeTypeImports, pi.srcID, dstID),
			Type:  graph.EdgeTypeImports,
			SrcID: pi.srcID,
			DstID: dstID,
		}); err != nil {
			warnings = append(warnings, err.Error())
		}
	}
	return warnings
}

// tsDependencyRoot derives the npm package name an import specifier
// belongs to: the specifier's first path segment, or its first two
// segments when scoped (e.g. "@nestjs/common" from
// "@nestjs/common/decorators").
func tsDependencyRoot(spec string) string {
	parts := strings.Split(spec, "/")
	if strings.HasPrefix(spec, "@") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// tsDecorator is a parsed `@Name(args)` decorator, args kept as raw text.
type tsDecorator struct {
	Name string
	Args string
}

// nestHTTPDecorators maps NestJS route decorators to HTTP methods.
var nestHTTPDecorators = map[string]string{
	"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE", "Patch": "PATCH",
}

// tsCallKeywords are identifiers that precede "(" but are language
// keywords, not calls.
var tsCallKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"function": true, "return": true, "new": true, "typeof": true, "await": true,
}

// Regex patterns for TypeScript/JavaScript parsing
var (
	tsImportRe        = regexp.MustCompile(`(?m)import\s+(?:{[^}]+}|[\w]+)\s+from\s+['"]([^'"]+)['"]`)
	tsNamedImportRe   = regexp.MustCompile(`(?m)import\s+\{([^}]+)\}\s+from\s+['"]([^'"]+)['"]`)
	tsRequireRe       = regexp.MustCompile(`(?m)(?:const|let|var)\s+(?:{[^}]+}|[\w]+)\s*=\s*require\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	tsClassRe         = regexp.MustCompile(`(?m)(?:export\s+)?(?:abstract\s+)?class\s+(\w+)(?:<[^>]+>)?(?:\s+extends\s+(\w+))?(?:\s+implements\s+([\w,\s]+))?\s*\{`)
	tsInterfaceRe     = regexp.MustCompile(`(?m)(?:export\s+)?interface\s+(\w+)(?:<[^>]+>)?(?:\s+extends\s+([\w,\s]+))?\s*\{`)
	tsTypeRe          = regexp.MustCompile(`(?m)(?:export\s+)?type\s+(\w+)(?:<[^>]+>)?\s*=`)
	tsEnumRe          = regexp.MustCompile(`(?m)(?:export\s+)?(?:const\s+)?enum\s+(\w+)\s*\{`)
	tsFunctionRe      = regexp.MustCompile(`(?m)(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\([^)]*\)`)
	tsMethodRe        = regexp.MustCompile(`(?m)(?:public|private|protected)?\s*(?:static\s+)?(?:async\s+)?(\w+)\s*\([^)]*\)\s*:[^{;]*`)
	tsFieldRe         = regexp.MustCompile(`(?m)(?:public|private|protected)?\s*(?:static\s+)?(?:readonly\s+)?(\w+)\s*:\s*([\w<>\[\],.]+)`)
	tsArrowFuncRe     = regexp.MustCompile(`(?m)(?:export\s+)?(?:const|let)\s+(\w+)\s*=\s*(?:async\s+)?\([^)]*\)\s*=>`)
	tsDecoratorNameRe = regexp.MustCompile(`(?m)^[ \t]*@(\w+)`)
	callExprRe        = regexp.MustCompile(`(\w+)\s*\(`)
	quotedRe          = regexp.MustCompile(`['"]([^'"]*)['"]`)
)

// extractFile parses a single TypeScript/JavaScript file and adds nodes/edges to the graph.
func (e *TypeScriptExtractor) extractFile(g *graph.Graph, filePath, content, repoPath string) ([]string, []pendingEdge, []pendingImport) {
	var warnings []string
	var pending []pendingEdge
	var pendingImports []pendingImport

	decoratorSpans := scanDecoratorSpans(content)

	// Determine module name from file path
	moduleName := e.getModuleName(filePath, repoPath)

	// Create package node for the module
	pkgID := graph.PackageID(moduleName)
	g.AddNode(&graph.Node{
		ID:   pkgID,
		Type: graph.NodeTypePackage,
		Properties: map[string]any{
			"name":     moduleName,
			"language": string(e.Language()),
		},
		Hash: graph.ContentHash(moduleName),
		File: filePath,
	})

	// Extract imports. Classification (internal Package vs external
	// ExternalDependency) is deferred to resolveImports, once the whole
	// walk's internal-module set is known — see pendingImport. A relative
	// specifier is resolved to the internal dotted-module scheme first, so
	// it can match a Package node; a non-relative one (an npm package name)
	// is left as-is.
	for _, matches := range tsImportRe.FindAllStringSubmatch(content, -1) {
		target := resolveImportModule(filePath, repoPath, matches[1])
		pendingImports = append(pendingImports, pendingImport{srcID: pkgID, target: target})
	}

	for _, matches := range tsRequireRe.FindAllStringSubmatch(content, -1) {
		target := resolveImportModule(filePath, repoPath, matches[1])
		pendingImports = append(pendingImports, pendingImport{srcID: pkgID, target: target})
	}

	// Map imported symbol names to their resolved module, so extends/
	// implements/@Inject targets defined in another file can be resolved.
	imports := map[string]string{}
	for _, matches := range tsNamedImportRe.FindAllStringSubmatch(content, -1) {
		resolved := resolveImportModule(filePath, repoPath, matches[2])
		for _, sym := range strings.Split(matches[1], ",") {
			sym = strings.TrimSpace(sym)
			if idx := strings.Index(sym, " as "); idx >= 0 {
				sym = strings.TrimSpace(sym[idx+4:])
			}
			if sym != "" {
				imports[sym] = resolved
			}
		}
	}

	// Extract classes
	for _, matches := range tsClassRe.FindAllStringSubmatch(content, -1) {
		className := matches[1]
		parentClass := matches[2]
		implementsStr := matches[3]

		classID := graph.StructID(moduleName, className)
		decs := parseDecorators(decoratorSpans, content, matches[0])

		props := map[string]any{
			"package":  moduleName,
			"name":     className,
			"language": string(e.Language()),
		}

		if strings.Contains(matches[0], "abstract") {
			props["abstract"] = true
		}
		for _, dec := range decs {
			props["decorator_"+dec.Name] = "true"
		}

		mapsToTable, tableName := entityTableName(decs, className)
		if mapsToTable {
			props["table"] = tableName
		}

		g.AddNode(&graph.Node{
			ID:         classID,
			Type:       graph.NodeTypeStruct,
			File:       filePath,
			LineStart:  findLineNumber(content, matches[0]),
			LineEnd:    findLineNumber(content, matches[0]) + countLines(matches[0]),
			Signature:  "class " + className,
			Hash:       graph.ContentHash(matches[0]),
			Properties: props,
		})

		// Handle extends
		if parentClass != "" {
			pending = append(pending, pendingEdge{graph.EdgeTypeExtends, classID, resolveType(parentClass, moduleName, imports)})
		}

		// Handle implements
		if implementsStr != "" {
			for _, iface := range strings.Split(implementsStr, ",") {
				iface = strings.TrimSpace(iface)
				if iface != "" {
					pending = append(pending, pendingEdge{graph.EdgeTypeImplements, classID, resolveType(iface, moduleName, imports)})
				}
			}
		}

		// TypeORM: @Entity() -> Table node + maps_to_table edge
		if mapsToTable {
			tableID := graph.TableID(tableName)
			g.AddNode(&graph.Node{
				ID:   tableID,
				Type: graph.NodeTypeTable,
				Properties: map[string]any{
					"name":     tableName,
					"language": string(e.Language()),
				},
				Hash: graph.ContentHash(tableID),
			})
			if err := g.AddEdge(&graph.Edge{
				ID:    graph.EdgeID(graph.EdgeTypeMapsToTable, classID, tableID),
				Type:  graph.EdgeTypeMapsToTable,
				SrcID: classID,
				DstID: tableID,
			}); err != nil {
				warnings = append(warnings, err.Error())
			}
		}

		// NestJS/generic decorators -> Decorator nodes + decorated edges.
		// @Entity is handled above via maps_to_table instead.
		basePath := ""
		for _, dec := range decs {
			if dec.Name == "Entity" {
				continue
			}
			if dec.Name == "Controller" {
				basePath = firstQuoted(dec.Args)
			}
			decID := graph.DecoratorID(moduleName, dec.Name)
			g.AddNode(&graph.Node{
				ID:   decID,
				Type: graph.NodeTypeDecorator,
				Properties: map[string]any{
					"name":     dec.Name,
					"language": string(e.Language()),
				},
				Hash: graph.ContentHash(decID),
			})
			if err := g.AddEdge(&graph.Edge{
				ID:    graph.EdgeID(graph.EdgeTypeDecorated, classID, decID),
				Type:  graph.EdgeTypeDecorated,
				SrcID: classID,
				DstID: decID,
			}); err != nil {
				warnings = append(warnings, err.Error())
			}
		}

		// Extract class body
		classBody := extractClassBody(content, matches[0])
		mWarnings, mPending := e.extractMethods(g, classBody, filePath, moduleName, classID, basePath)
		warnings = append(warnings, mWarnings...)
		pending = append(pending, mPending...)

		fWarnings, fPending := e.extractFields(g, classBody, filePath, moduleName, classID, tableName, imports)
		warnings = append(warnings, fWarnings...)
		pending = append(pending, fPending...)
	}

	// Extract interfaces
	for _, matches := range tsInterfaceRe.FindAllStringSubmatch(content, -1) {
		ifaceName := matches[1]
		extendsStr := matches[2]
		ifaceID := graph.InterfaceID(moduleName, ifaceName)

		g.AddNode(&graph.Node{
			ID:        ifaceID,
			Type:      graph.NodeTypeInterface,
			File:      filePath,
			LineStart: findLineNumber(content, matches[0]),
			LineEnd:   findLineNumber(content, matches[0]) + countLines(matches[0]),
			Signature: "interface " + ifaceName,
			Hash:      graph.ContentHash(matches[0]),
			Properties: map[string]any{
				"package":  moduleName,
				"name":     ifaceName,
				"language": string(e.Language()),
			},
		})

		if extendsStr != "" {
			for _, parent := range strings.Split(extendsStr, ",") {
				parent = strings.TrimSpace(parent)
				if parent != "" {
					pending = append(pending, pendingEdge{graph.EdgeTypeExtends, ifaceID, resolveType(parent, moduleName, imports)})
				}
			}
		}
	}

	// Extract type aliases
	for _, matches := range tsTypeRe.FindAllStringSubmatch(content, -1) {
		typeName := matches[1]
		typeID := graph.TypeAliasID(moduleName, typeName)

		g.AddNode(&graph.Node{
			ID:        typeID,
			Type:      graph.NodeTypeTypeAlias,
			File:      filePath,
			LineStart: findLineNumber(content, matches[0]),
			LineEnd:   findLineNumber(content, matches[0]) + countLines(matches[0]),
			Signature: "type " + typeName,
			Hash:      graph.ContentHash(matches[0]),
			Properties: map[string]any{
				"package":  moduleName,
				"name":     typeName,
				"language": string(e.Language()),
			},
		})
	}

	// Extract enums
	for _, matches := range tsEnumRe.FindAllStringSubmatch(content, -1) {
		enumName := matches[1]
		enumID := graph.EnumID(moduleName, enumName)

		g.AddNode(&graph.Node{
			ID:        enumID,
			Type:      graph.NodeTypeEnum,
			File:      filePath,
			LineStart: findLineNumber(content, matches[0]),
			LineEnd:   findLineNumber(content, matches[0]) + countLines(matches[0]),
			Signature: "enum " + enumName,
			Hash:      graph.ContentHash(matches[0]),
			Properties: map[string]any{
				"package":  moduleName,
				"name":     enumName,
				"language": string(e.Language()),
			},
		})
	}

	// Extract standalone functions
	for _, matches := range tsFunctionRe.FindAllStringSubmatch(content, -1) {
		funcName := matches[1]
		funcID := graph.FunctionID(moduleName, "", funcName)
		body := braceBodyAfterMatch(content, matches[0])

		g.AddNode(&graph.Node{
			ID:        funcID,
			Type:      graph.NodeTypeFunction,
			File:      filePath,
			LineStart: findLineNumber(content, matches[0]),
			LineEnd:   findLineNumber(content, matches[0]) + countLines(matches[0]) + countLines(body),
			Signature: funcName + "()",
			Hash:      graph.ContentHash(matches[0] + body),
			Properties: map[string]any{
				"package":  moduleName,
				"name":     funcName,
				"language": string(e.Language()),
			},
		})

		for _, callee := range extractCalledNames(body) {
			calleeID := graph.FunctionID(moduleName, "", callee)
			if calleeID == funcID {
				continue
			}
			pending = append(pending, pendingEdge{graph.EdgeTypeCalls, funcID, calleeID})
		}
	}

	// Extract arrow functions
	for _, matches := range tsArrowFuncRe.FindAllStringSubmatch(content, -1) {
		funcName := matches[1]
		funcID := graph.FunctionID(moduleName, "", funcName)
		body := braceBodyAfterMatch(content, matches[0])

		g.AddNode(&graph.Node{
			ID:        funcID,
			Type:      graph.NodeTypeFunction,
			File:      filePath,
			LineStart: findLineNumber(content, matches[0]),
			LineEnd:   findLineNumber(content, matches[0]) + countLines(matches[0]) + countLines(body),
			Signature: funcName + "()",
			Hash:      graph.ContentHash(matches[0] + body),
			Properties: map[string]any{
				"package":  moduleName,
				"name":     funcName,
				"language": string(e.Language()),
			},
		})

		for _, callee := range extractCalledNames(body) {
			calleeID := graph.FunctionID(moduleName, "", callee)
			if calleeID == funcID {
				continue
			}
			pending = append(pending, pendingEdge{graph.EdgeTypeCalls, funcID, calleeID})
		}
	}

	return warnings, pending, pendingImports
}

// entityTableName inspects a class's decorators for a TypeORM @Entity() and
// returns whether it maps to a table, plus the table name (from a quoted
// argument when given, falling back to the lowercased class name).
func entityTableName(decs []tsDecorator, className string) (mapsToTable bool, tableName string) {
	for _, d := range decs {
		if d.Name == "Entity" {
			mapsToTable = true
			tableName = firstQuoted(d.Args)
		}
	}
	if mapsToTable && tableName == "" {
		tableName = strings.ToLower(className)
	}
	return mapsToTable, tableName
}

// mappingFromDecorators returns the HTTP method and path segment declared
// by a NestJS route decorator (@Get/@Post/etc), if any is present.
func mappingFromDecorators(decs []tsDecorator) (method, path string, ok bool) {
	for _, d := range decs {
		if m, found := nestHTTPDecorators[d.Name]; found {
			return m, firstQuoted(d.Args), true
		}
	}
	return "", "", false
}

// joinRoute joins a class-level base path with a method-level path segment
// into a single NestJS route, e.g. ("users", ":id") -> "users/:id".
func joinRoute(base, sub string) string {
	base = strings.Trim(base, "/")
	sub = strings.Trim(sub, "/")
	switch {
	case base == "":
		return sub
	case sub == "":
		return base
	default:
		return base + "/" + sub
	}
}

// extractMethods extracts methods from a class body, including NestJS route
// decorators (-> Endpoint nodes/routed edges) and best-effort call edges.
func (e *TypeScriptExtractor) extractMethods(g *graph.Graph, content, filePath, moduleName, classID, basePath string) ([]string, []pendingEdge) {
	var warnings []string
	var pending []pendingEdge

	decoratorSpans := scanDecoratorSpans(content)

	for _, matches := range tsMethodRe.FindAllStringSubmatch(content, -1) {
		methodName := matches[1]
		if methodName == "constructor" || methodName == "class" || methodName == "interface" {
			continue
		}

		funcID := graph.FunctionID(moduleName, "", methodName)
		decs := parseDecorators(decoratorSpans, content, matches[0])
		body := braceBodyAfterMatch(content, matches[0])

		props := map[string]any{
			"package":  moduleName,
			"name":     methodName,
			"language": string(e.Language()),
		}

		if strings.Contains(matches[0], "static") {
			props["static"] = true
		}
		for _, dec := range decs {
			props["decorator_"+dec.Name] = "true"
		}

		g.AddNode(&graph.Node{
			ID:         funcID,
			Type:       graph.NodeTypeFunction,
			File:       filePath,
			LineStart:  findLineNumber(content, matches[0]),
			LineEnd:    findLineNumber(content, matches[0]) + countLines(matches[0]) + countLines(body),
			Signature:  methodName + "()",
			Hash:       graph.ContentHash(matches[0] + body),
			Properties: props,
		})

		if err := g.AddEdge(&graph.Edge{
			ID:    graph.EdgeID(graph.EdgeTypeHasMethod, classID, funcID),
			Type:  graph.EdgeTypeHasMethod,
			SrcID: classID,
			DstID: funcID,
		}); err != nil {
			warnings = append(warnings, err.Error())
		}

		if httpMethod, path, ok := mappingFromDecorators(decs); ok {
			fullPath := joinRoute(basePath, path)
			epID := graph.EndpointID(httpMethod, fullPath)
			g.AddNode(&graph.Node{
				ID:   epID,
				Type: graph.NodeTypeEndpoint,
				Properties: map[string]any{
					"method":   httpMethod,
					"path":     fullPath,
					"language": string(e.Language()),
				},
				Hash: graph.ContentHash(epID),
			})
			if err := g.AddEdge(&graph.Edge{
				ID:    graph.EdgeID(graph.EdgeTypeRouted, funcID, epID),
				Type:  graph.EdgeTypeRouted,
				SrcID: funcID,
				DstID: epID,
			}); err != nil {
				warnings = append(warnings, err.Error())
			}
		}

		for _, callee := range extractCalledNames(body) {
			calleeID := graph.FunctionID(moduleName, "", callee)
			if calleeID == funcID {
				continue
			}
			pending = append(pending, pendingEdge{graph.EdgeTypeCalls, funcID, calleeID})
		}
	}
	return warnings, pending
}

// extractFields extracts fields from a class body. When tableName is
// non-empty (the owning class is a TypeORM @Entity), each field also
// becomes a Column node under that table (Decision 8).
func (e *TypeScriptExtractor) extractFields(g *graph.Graph, content, filePath, moduleName, classID, tableName string, imports map[string]string) ([]string, []pendingEdge) {
	var warnings []string
	var pending []pendingEdge

	decoratorSpans := scanDecoratorSpans(content)

	var tableID string
	if tableName != "" {
		tableID = graph.TableID(tableName)
	}

	for _, matches := range tsFieldRe.FindAllStringSubmatch(content, -1) {
		fieldName := matches[1]
		fieldType := matches[2]
		fieldID := graph.FieldID(classID, fieldName)
		decs := parseDecorators(decoratorSpans, content, matches[0])

		props := map[string]any{
			"package":  moduleName,
			"name":     fieldName,
			"type":     fieldType,
			"language": string(e.Language()),
		}

		if strings.Contains(matches[0], "static") {
			props["static"] = true
		}

		columnName := ""
		for _, dec := range decs {
			switch dec.Name {
			case "Inject":
				pending = append(pending, pendingEdge{graph.EdgeTypeInjected, classID, resolveType(fieldType, moduleName, imports)})
			case "Column":
				columnName = firstQuoted(dec.Args)
			}
		}

		if tableID != "" {
			if columnName == "" {
				columnName = fieldName
			}
			props["column"] = columnName
		}

		g.AddNode(&graph.Node{
			ID:         fieldID,
			Type:       graph.NodeTypeField,
			File:       filePath,
			LineStart:  findLineNumber(content, matches[0]),
			LineEnd:    findLineNumber(content, matches[0]) + countLines(matches[0]),
			Signature:  fieldName,
			Hash:       graph.ContentHash(matches[0]),
			Properties: props,
		})

		if err := g.AddEdge(&graph.Edge{
			ID:    graph.EdgeID(graph.EdgeTypeHasField, classID, fieldID),
			Type:  graph.EdgeTypeHasField,
			SrcID: classID,
			DstID: fieldID,
		}); err != nil {
			warnings = append(warnings, err.Error())
		}

		if tableID != "" {
			colID := graph.ColumnID(tableName, columnName)
			g.AddNode(&graph.Node{
				ID:   colID,
				Type: graph.NodeTypeColumn,
				Properties: map[string]any{
					"name":     columnName,
					"table":    tableName,
					"language": string(e.Language()),
				},
				Hash: graph.ContentHash(colID),
			})
			if err := g.AddEdge(&graph.Edge{
				ID:    graph.EdgeID(graph.EdgeTypeHasField, tableID, colID),
				Type:  graph.EdgeTypeHasField,
				SrcID: tableID,
				DstID: colID,
			}); err != nil {
				warnings = append(warnings, err.Error())
			}
		}
	}
	return warnings, pending
}

// extractDependencies extracts dependencies from package.json.
func (e *TypeScriptExtractor) extractDependencies(repoPath string, g *graph.Graph) []string {
	var warnings []string

	packageJSONPath := filepath.Join(repoPath, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		content, readErr := os.ReadFile(packageJSONPath)
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("package.json: %v", readErr))
		} else {
			extractNPMDependencies(string(content), g, e.Language())
		}
	}

	return warnings
}

// extractNPMDependencies extracts dependencies from package.json.
func extractNPMDependencies(content string, g *graph.Graph, lang common.Language) {
	// Simple regex-based extraction
	depRe := regexp.MustCompile(`"([^"]+)"\s*:\s*"[^"]+"`)

	// Find dependencies section
	depIdx := strings.Index(content, `"dependencies"`)
	if depIdx > 0 {
		section := content[depIdx:]
		endIdx := strings.Index(section, "}")
		if endIdx > 0 {
			section = section[:endIdx]
			for _, matches := range depRe.FindAllStringSubmatch(section, -1) {
				pkgName := matches[1]
				depID := graph.ExternalDependencyID(pkgName)
				g.AddNode(&graph.Node{
					ID:   depID,
					Type: graph.NodeTypeExternalDependency,
					Properties: map[string]any{
						"name":     pkgName,
						"language": string(lang),
					},
					Hash: graph.ContentHash(pkgName),
				})
			}
		}
	}

	// Find devDependencies section
	devDepIdx := strings.Index(content, `"devDependencies"`)
	if devDepIdx > 0 {
		section := content[devDepIdx:]
		endIdx := strings.Index(section, "}")
		if endIdx > 0 {
			section = section[:endIdx]
			for _, matches := range depRe.FindAllStringSubmatch(section, -1) {
				pkgName := matches[1]
				depID := graph.ExternalDependencyID(pkgName)
				g.AddNode(&graph.Node{
					ID:   depID,
					Type: graph.NodeTypeExternalDependency,
					Properties: map[string]any{
						"name":     pkgName,
						"language": string(lang),
						"dev":      true,
					},
					Hash: graph.ContentHash(pkgName),
				})
			}
		}
	}
}

// getModuleName determines the module name from file path.
func (e *TypeScriptExtractor) getModuleName(filePath, repoPath string) string {
	rel, err := filepath.Rel(repoPath, filePath)
	if err != nil {
		return filepath.Base(filePath)
	}

	// Remove extension
	ext := filepath.Ext(rel)
	moduleName := strings.TrimSuffix(rel, ext)

	// Convert path separators to dots
	moduleName = strings.ReplaceAll(moduleName, string(filepath.Separator), ".")

	// Remove index suffix
	moduleName = strings.TrimSuffix(moduleName, ".index")

	return moduleName
}

// decoratorSpan is a single `@Name(args)` decorator located in some text,
// with the byte range it occupies (args consumed via balanced-paren
// scanning, so a multi-line decorator is captured as one span).
type decoratorSpan struct {
	start, end int
	dec        tsDecorator
}

// scanDecoratorSpans finds every decorator declaration in content. Each
// decorator's optional parenthesized arguments are consumed with
// common.ScanBalanced rather than a same-line regex, so arguments spanning
// multiple lines (e.g. a multi-line `@Get(...)`) are captured intact.
func scanDecoratorSpans(content string) []decoratorSpan {
	var spans []decoratorSpan
	for _, m := range tsDecoratorNameRe.FindAllStringSubmatchIndex(content, -1) {
		matchEnd, nameStart, nameEnd := m[1], m[2], m[3]
		start := nameStart - 1 // position of "@", immediately before the name
		name := content[nameStart:nameEnd]
		end := matchEnd
		args := ""

		i := matchEnd
		for i < len(content) && (content[i] == ' ' || content[i] == '\t') {
			i++
		}
		if i < len(content) && content[i] == '(' {
			if inner, argEnd, ok := common.ScanBalanced(content, i, '(', ')'); ok {
				args = inner
				end = argEnd
			}
		}

		spans = append(spans, decoratorSpan{start: start, end: end, dec: tsDecorator{Name: name, Args: args}})
	}
	return spans
}

// parseDecorators returns the decorators, in source order, forming the
// contiguous run immediately preceding declaration — i.e. every span in
// spans whose end is separated from the next decorator's start (or from
// declaration itself) by nothing but whitespace.
func parseDecorators(spans []decoratorSpan, content, declaration string) []tsDecorator {
	idx := strings.Index(content, declaration)
	if idx <= 0 {
		return nil
	}

	var decorators []tsDecorator
	cursor := idx
	for i := len(spans) - 1; i >= 0; i-- {
		sp := spans[i]
		if sp.end > cursor {
			continue
		}
		if strings.TrimSpace(content[sp.end:cursor]) != "" {
			break
		}
		decorators = append(decorators, sp.dec)
		cursor = sp.start
	}

	for i, j := 0, len(decorators)-1; i < j; i, j = i+1, j-1 {
		decorators[i], decorators[j] = decorators[j], decorators[i]
	}
	return decorators
}

// firstQuoted returns the first quoted string literal in s (single or
// double quotes), e.g. `'users'` -> "users". Returns "" if none is found.
func firstQuoted(s string) string {
	m := quotedRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

// extractCalledNames does a best-effort scan of a function/method body for
// call expressions (`name(`, `obj.name(`, `this.name(`), skipping
// control-flow keywords that also precede "(".
func extractCalledNames(body string) []string {
	var names []string
	for _, m := range callExprRe.FindAllStringSubmatch(body, -1) {
		name := m[1]
		if tsCallKeywords[name] {
			continue
		}
		names = append(names, name)
	}
	return names
}

// braceBodyAfterMatch returns the text between the braces of a block body
// starting right after declaration, or "" if no "{" immediately follows
// (e.g. an expression-bodied arrow function, or a method without a body).
func braceBodyAfterMatch(content, declaration string) string {
	start := strings.Index(content, declaration)
	if start < 0 {
		return ""
	}
	i := start + len(declaration)
	for i < len(content) && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r') {
		i++
	}
	if i >= len(content) || content[i] != '{' {
		return ""
	}

	depth := 0
	for j := i; j < len(content); j++ {
		switch content[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[i+1 : j]
			}
		}
	}
	return content[i+1:]
}

// extractClassBody extracts the body of a class declaration.
func extractClassBody(content, classDecl string) string {
	startIdx := strings.Index(content, classDecl)
	if startIdx < 0 {
		return ""
	}

	braceIdx := strings.Index(content[startIdx:], "{")
	if braceIdx < 0 {
		return ""
	}

	depth := 0
	bodyStart := startIdx + braceIdx
	for i := bodyStart; i < len(content); i++ {
		if content[i] == '{' {
			depth++
		} else if content[i] == '}' {
			depth--
			if depth == 0 {
				return content[bodyStart+1 : i]
			}
		}
	}

	return ""
}

// resolveType resolves a type name to its full ID, preferring a named
// import's resolved module over assuming the current module.
func resolveType(typeName, currentModule string, imports map[string]string) string {
	if strings.Contains(typeName, ".") {
		return typeName
	}
	if mod, ok := imports[typeName]; ok {
		return mod + "." + typeName
	}
	return currentModule + "." + typeName
}

// resolveImportModule resolves an import specifier to the internal module
// name scheme used elsewhere (repo-relative path with "." separators).
// Non-relative specifiers (external packages) are returned as-is; they
// won't match an internal node, so references to them are silently
// skipped, same as any other unresolved best-effort target.
func resolveImportModule(filePath, repoPath, importPath string) string {
	if !strings.HasPrefix(importPath, ".") {
		return importPath
	}
	abs := filepath.Join(filepath.Dir(filePath), importPath)
	rel, err := filepath.Rel(repoPath, abs)
	if err != nil {
		return importPath
	}
	rel = strings.ReplaceAll(rel, string(filepath.Separator), ".")
	rel = strings.TrimSuffix(rel, ".index")
	return rel
}

// findLineNumber finds the line number of a substring in content.
func findLineNumber(content, substr string) int {
	idx := strings.Index(content, substr)
	if idx < 0 {
		return 1
	}
	line := 1
	for i := 0; i < idx; i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return line
}

// countLines counts the number of lines in a string.
func countLines(s string) int {
	return strings.Count(s, "\n") + 1
}
