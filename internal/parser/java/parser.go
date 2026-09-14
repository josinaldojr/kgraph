// Package javaparser implements the common.Extractor interface for Java
// source code. This implementation uses regex-based parsing for now.
// Tree-sitter support can be added later when CGO is available.
package javaparser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/josinaldojr/kgraph/internal/graph"
	"github.com/josinaldojr/kgraph/internal/parser/common"
)

// JavaExtractor implements common.Extractor for Java source code.
type JavaExtractor struct{}

// NewJavaExtractor creates a new JavaExtractor.
func NewJavaExtractor() *JavaExtractor {
	return &JavaExtractor{}
}

// Language returns the language this extractor handles.
func (e *JavaExtractor) Language() common.Language {
	return common.LangJava
}

// FileExtensions returns the file extensions this extractor processes.
func (e *JavaExtractor) FileExtensions() []string {
	return []string{".java"}
}

// pendingEdge is an edge whose destination node may be defined in a file
// this extractor hasn't visited yet (e.g. extends/implements a class from
// another file, or a best-effort call/injection target). It is resolved
// after the whole repo has been walked and every node exists.
type pendingEdge struct {
	edgeType graph.EdgeType
	srcID    string
	dstID    string
}

// ExtractRepo extracts the full graph from a Java repository at repoPath.
func (e *JavaExtractor) ExtractRepo(repoPath string) (*graph.Graph, []string, error) {
	g := graph.New()
	warnings := e.extractDependencies(repoPath, g)

	walkWarnings, err := e.extractWalk(g, repoPath, nil, func(string) bool { return true })
	warnings = append(warnings, walkWarnings...)
	if err != nil {
		return nil, warnings, err
	}

	return g, warnings, nil
}

// ExtractPackages extracts a scoped subset of Java packages: only files
// under the given patterns (directories, relative to repoPath, e.g. "."
// or "./src/main/java/com/example") are walked. An empty patterns slice
// yields an empty graph. knownInternal (package paths already known from
// a prior build) augments the internal-package set used to classify
// import targets as internal vs external, since a scoped walk alone
// cannot discover packages outside patterns.
func (e *JavaExtractor) ExtractPackages(repoPath string, patterns []string, knownInternal map[string]bool) (*graph.Graph, []string, error) {
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

// extractWalk walks repoPath, running extractFile on every .java file
// whose directory (relative to repoPath, slash-normalized) include
// accepts, then resolves cross-file references — pending extends/
// implements/injected/calls edges, then import edges — against the
// resulting graph. Deferring both until after the walk means resolution
// doesn't depend on filesystem walk order and can classify a reference as
// internal or external using the complete internal-package set this pass
// produces (merged with knownInternal, for a scoped walk that can't see
// packages outside its own patterns).
func (e *JavaExtractor) extractWalk(g *graph.Graph, repoPath string, knownInternal map[string]bool, include func(relDir string) bool) ([]string, error) {
	var warnings []string
	var pending []pendingEdge
	var pendingImports []pendingImport

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" || name == "target" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".java") {
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
		return warnings, fmt.Errorf("java parser: walking %s: %w", repoPath, err)
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
	warnings = append(warnings, resolveImports(g, pendingImports, internal)...)

	return warnings, nil
}

// resolvePendingEdges adds every pending edge whose destination node now
// exists in the graph. A missing destination (an external/third-party
// type, or a call/injection target we can't resolve) is not an error —
// call/reference resolution here is best-effort, so it is skipped silently.
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

// pendingImport is an `import` statement's target, deferred (like
// pendingEdge) until the whole walk completes and the internal-package set
// is known, since classifying it as internal vs external can't be decided
// file-by-file. target is the raw import path (may end in ".*" for a
// wildcard import, or be static, e.g. an inner class or static member).
type pendingImport struct {
	srcID  string
	target string
}

// resolveImports creates an `imports` edge for every pending import,
// classifying its target's package as internal (present in internal) or
// external — mirroring the Go extractor's own internal/external
// classification for import edges. An internal target gets an edge to
// that Package node. An external target gets an edge to an
// ExternalDependency node: one a dependency-manifest parse already created
// is reused when the import's package root (its first two dotted
// components, e.g. "org.springframework" from
// "org.springframework.stereotype.Service") matches a declared Maven/
// Gradle dependency, otherwise a new node (keyed by that root) is created
// on the fly — e.g. for a JDK import with no manifest entry.
func resolveImports(g *graph.Graph, imports []pendingImport, internal map[string]bool) []string {
	var warnings []string
	for _, pi := range imports {
		pkg := javaImportPackage(pi.target)
		if pkg == "" {
			continue
		}

		var dstID string
		if internal[pkg] {
			dstID = graph.PackageID(pkg)
		} else {
			root := javaDependencyRoot(pkg)
			dstID = graph.ExternalDependencyID(root)
			if g.Node(dstID) == nil {
				g.AddNode(&graph.Node{
					ID:   dstID,
					Type: graph.NodeTypeExternalDependency,
					Properties: map[string]any{
						"name":     root,
						"language": string(common.LangJava),
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

// javaImportPackage returns the package an import statement's target
// belongs to: the target itself minus its trailing ".*" for a wildcard
// import, or minus its last dotted component (assumed to be the imported
// class/member name) otherwise.
func javaImportPackage(target string) string {
	if strings.HasSuffix(target, ".*") {
		return strings.TrimSuffix(target, ".*")
	}
	if idx := strings.LastIndex(target, "."); idx >= 0 {
		return target[:idx]
	}
	return target
}

// javaDependencyRoot derives a coarse "library root" from a package path
// for classifying it against Maven-style groupId conventions (e.g.
// "org.springframework" from "org.springframework.stereotype") — a
// heuristic, not an exact groupId, since no reliable mapping from Java
// package to Maven coordinate exists without the dependency's POM.
func javaDependencyRoot(pkg string) string {
	parts := strings.Split(pkg, ".")
	if len(parts) <= 2 {
		return pkg
	}
	return strings.Join(parts[:2], ".")
}

// javaAnnotation is a parsed `@Name(args)` annotation, with args kept as
// raw, unparsed text (e.g. `name = "users"`).
type javaAnnotation struct {
	Name string
	Args string
}

// httpMappingAnnotations maps Spring mapping annotations to HTTP methods.
var httpMappingAnnotations = map[string]string{
	"GetMapping":    "GET",
	"PostMapping":   "POST",
	"PutMapping":    "PUT",
	"DeleteMapping": "DELETE",
	"PatchMapping":  "PATCH",
}

// javaCallKeywords are identifiers that precede "(" but are language
// keywords, not calls, and must be excluded from best-effort call
// extraction.
var javaCallKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"synchronized": true, "return": true, "new": true, "super": true, "this": true,
}

// Regex patterns for Java parsing
var (
	packageRe        = regexp.MustCompile(`(?m)^package\s+([\w.]+)\s*;`)
	importRe         = regexp.MustCompile(`(?m)^import\s+(?:static\s+)?([\w.*]+)\s*;`)
	classRe          = regexp.MustCompile(`(?m)(?:public\s+)?(?:abstract\s+)?(?:class)\s+(\w+)(?:\s+extends\s+(\w+))?(?:\s+implements\s+([\w,\s]+))?\s*\{`)
	interfaceRe      = regexp.MustCompile(`(?m)(?:public\s+)?interface\s+(\w+)(?:\s+extends\s+([\w,\s]+))?\s*\{`)
	enumRe           = regexp.MustCompile(`(?m)(?:public\s+)?enum\s+(\w+)\s*\{`)
	methodRe         = regexp.MustCompile(`(?m)(?:public|private|protected)?\s*(?:static\s+)?(?:abstract\s+)?(?:[\w<>\[\],\s]+)\s+(\w+)\s*\([^)]*\)\s*(?:throws\s+[\w,\s]+)?\s*\{?`)
	fieldRe          = regexp.MustCompile(`(?m)(?:public|private|protected)?\s*(?:static\s+)?(?:final\s+)?([\w<>\[\],]+)\s+(\w+)\s*(?:=\s*[^;]+)?\s*;`)
	annotationNameRe = regexp.MustCompile(`(?m)^[ \t]*@(\w+)`)
	callExprRe       = regexp.MustCompile(`(\w+)\s*\(`)
	quotedRe         = regexp.MustCompile(`"([^"]*)"`)
)

// extractFile parses a single Java file and adds nodes/edges to the graph.
// It returns warnings plus a set of edges that must be resolved only after
// every file in the repo has been processed (see pendingEdge).
func (e *JavaExtractor) extractFile(g *graph.Graph, filePath, content, repoPath string) ([]string, []pendingEdge, []pendingImport) {
	var warnings []string
	var pending []pendingEdge
	var pendingImports []pendingImport

	annotationSpans := scanAnnotationSpans(content)

	// Extract package
	pkgName := ""
	if matches := packageRe.FindStringSubmatch(content); matches != nil {
		pkgName = matches[1]
	}
	if pkgName != "" {
		g.AddNode(&graph.Node{
			ID:   graph.PackageID(pkgName),
			Type: graph.NodeTypePackage,
			File: filePath,
			Properties: map[string]any{
				"name":     pkgName,
				"language": string(common.LangJava),
			},
			Hash: graph.ContentHash(pkgName),
		})
	}

	// Extract imports. imports (bare paths) feeds resolveType's base-class/
	// interface/field-type resolution; pendingImports feeds resolveImports,
	// which classifies each target as internal vs external once the whole
	// walk's internal-package set is known (see pendingImport).
	var imports []string
	for _, matches := range importRe.FindAllStringSubmatch(content, -1) {
		imports = append(imports, matches[1])
		if pkgName != "" {
			pendingImports = append(pendingImports, pendingImport{srcID: graph.PackageID(pkgName), target: matches[1]})
		}
	}

	// Extract classes
	for _, matches := range classRe.FindAllStringSubmatch(content, -1) {
		className := matches[1]
		parentClass := matches[2]
		implementsStr := matches[3]

		classID := graph.StructID(pkgName, className)
		anns := parseAnnotations(annotationSpans, content, matches[0])

		props := map[string]any{
			"package":  pkgName,
			"name":     className,
			"language": string(common.LangJava),
		}

		// Check for abstract
		if strings.Contains(matches[0], "abstract") {
			props["abstract"] = true
		}
		for _, ann := range anns {
			props["annotation_"+ann.Name] = "true"
		}

		mapsToTable, tableName := entityTableName(anns, className)
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
			pending = append(pending, pendingEdge{graph.EdgeTypeExtends, classID, resolveType(parentClass, pkgName, imports)})
		}

		// Handle implements
		if implementsStr != "" {
			for _, iface := range strings.Split(implementsStr, ",") {
				iface = strings.TrimSpace(iface)
				if iface != "" {
					pending = append(pending, pendingEdge{graph.EdgeTypeImplements, classID, resolveType(iface, pkgName, imports)})
				}
			}
		}

		// JPA: @Entity/@Table -> Table node + maps_to_table edge
		if mapsToTable {
			tableID := graph.TableID(tableName)
			g.AddNode(&graph.Node{
				ID:   tableID,
				Type: graph.NodeTypeTable,
				Properties: map[string]any{
					"name":     tableName,
					"language": string(common.LangJava),
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

		// Spring/generic annotations -> Decorator nodes + decorated edges.
		// @Entity/@Table are handled above via maps_to_table instead.
		basePath := ""
		for _, ann := range anns {
			if ann.Name == "Entity" || ann.Name == "Table" {
				continue
			}
			if ann.Name == "RequestMapping" {
				if p := firstQuoted(ann.Args); p != "" {
					basePath = p
				}
			}
			decID := graph.DecoratorID(pkgName, ann.Name)
			g.AddNode(&graph.Node{
				ID:   decID,
				Type: graph.NodeTypeDecorator,
				Properties: map[string]any{
					"name":     ann.Name,
					"language": string(common.LangJava),
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

		// Extract methods and fields from class body
		classBody := extractClassBody(content, matches[0])
		mWarnings, mPending := extractMethods(g, classBody, filePath, pkgName, classID, basePath)
		warnings = append(warnings, mWarnings...)
		pending = append(pending, mPending...)

		fieldTableName := ""
		if mapsToTable {
			fieldTableName = tableName
		}
		fWarnings, fPending := extractFields(g, classBody, filePath, pkgName, classID, imports, fieldTableName)
		warnings = append(warnings, fWarnings...)
		pending = append(pending, fPending...)
	}

	// Extract interfaces
	for _, matches := range interfaceRe.FindAllStringSubmatch(content, -1) {
		ifaceName := matches[1]
		extendsStr := matches[2]
		ifaceID := graph.InterfaceID(pkgName, ifaceName)

		g.AddNode(&graph.Node{
			ID:        ifaceID,
			Type:      graph.NodeTypeInterface,
			File:      filePath,
			LineStart: findLineNumber(content, matches[0]),
			LineEnd:   findLineNumber(content, matches[0]) + countLines(matches[0]),
			Signature: "interface " + ifaceName,
			Hash:      graph.ContentHash(matches[0]),
			Properties: map[string]any{
				"package":  pkgName,
				"name":     ifaceName,
				"language": string(common.LangJava),
			},
		})

		if extendsStr != "" {
			for _, parent := range strings.Split(extendsStr, ",") {
				parent = strings.TrimSpace(parent)
				if parent != "" {
					pending = append(pending, pendingEdge{graph.EdgeTypeExtends, ifaceID, resolveType(parent, pkgName, imports)})
				}
			}
		}
	}

	// Extract enums
	for _, matches := range enumRe.FindAllStringSubmatch(content, -1) {
		enumName := matches[1]
		enumID := graph.EnumID(pkgName, enumName)

		g.AddNode(&graph.Node{
			ID:        enumID,
			Type:      graph.NodeTypeEnum,
			File:      filePath,
			LineStart: findLineNumber(content, matches[0]),
			LineEnd:   findLineNumber(content, matches[0]) + countLines(matches[0]),
			Signature: "enum " + enumName,
			Hash:      graph.ContentHash(matches[0]),
			Properties: map[string]any{
				"package":  pkgName,
				"name":     enumName,
				"language": string(common.LangJava),
			},
		})
	}

	return warnings, pending, pendingImports
}

// entityTableName inspects a class's annotations for @Entity and returns
// whether the class maps to a table, plus the table name (from an
// accompanying @Table(name=...) when given, falling back to the lowercased
// class name). @Table alone, without @Entity, does not map a class to a
// table — per JPA, @Table only customizes the table for an already-@Entity
// class.
func entityTableName(anns []javaAnnotation, className string) (mapsToTable bool, tableName string) {
	hasEntity := false
	for _, a := range anns {
		switch a.Name {
		case "Entity":
			hasEntity = true
		case "Table":
			if n := firstQuoted(a.Args); n != "" {
				tableName = n
			}
		}
	}
	mapsToTable = hasEntity
	if mapsToTable && tableName == "" {
		tableName = strings.ToLower(className)
	}
	return mapsToTable, tableName
}

// mappingFromAnnotations returns the HTTP method and path segment declared
// by a Spring `*Mapping` annotation, if any is present.
func mappingFromAnnotations(anns []javaAnnotation) (method, path string, ok bool) {
	for _, a := range anns {
		if m, found := httpMappingAnnotations[a.Name]; found {
			return m, firstQuoted(a.Args), true
		}
	}
	return "", "", false
}

// joinPath joins a class-level base path with a method-level path segment
// into a single route, e.g. ("/api/users", "/{id}") -> "/api/users/{id}".
func joinPath(base, sub string) string {
	base = strings.TrimSuffix(base, "/")
	if sub == "" {
		if base == "" {
			return "/"
		}
		return base
	}
	if !strings.HasPrefix(sub, "/") {
		sub = "/" + sub
	}
	return base + sub
}

// extractMethods extracts methods from a class body, including Spring route
// annotations (-> Endpoint nodes/routed edges) and best-effort call edges.
func extractMethods(g *graph.Graph, content, filePath, pkgName, classID, basePath string) ([]string, []pendingEdge) {
	var warnings []string
	var pending []pendingEdge

	annotationSpans := scanAnnotationSpans(content)

	for _, matches := range methodRe.FindAllStringSubmatch(content, -1) {
		methodName := matches[1]
		if methodName == "class" || methodName == "interface" || methodName == "enum" {
			continue
		}

		funcID := graph.FunctionID(pkgName, "", methodName)
		anns := parseAnnotations(annotationSpans, content, matches[0])
		body := methodBody(content, matches[0])

		props := map[string]any{
			"package":  pkgName,
			"name":     methodName,
			"language": string(common.LangJava),
		}

		if strings.Contains(matches[0], "static") {
			props["static"] = true
		}
		for _, ann := range anns {
			props["annotation_"+ann.Name] = "true"
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

		if httpMethod, path, ok := mappingFromAnnotations(anns); ok {
			fullPath := joinPath(basePath, path)
			epID := graph.EndpointID(httpMethod, fullPath)
			g.AddNode(&graph.Node{
				ID:   epID,
				Type: graph.NodeTypeEndpoint,
				Properties: map[string]any{
					"method":   httpMethod,
					"path":     fullPath,
					"language": string(common.LangJava),
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
			calleeID := graph.FunctionID(pkgName, "", callee)
			if calleeID == funcID {
				continue
			}
			pending = append(pending, pendingEdge{graph.EdgeTypeCalls, funcID, calleeID})
		}
	}
	return warnings, pending
}

// extractFields extracts fields from a class body. When tableName is
// non-empty (the owning class is a JPA @Entity), each field also becomes a
// Column node under that table (Decision 8).
func extractFields(g *graph.Graph, content, filePath, pkgName, classID string, imports []string, tableName string) ([]string, []pendingEdge) {
	var warnings []string
	var pending []pendingEdge

	annotationSpans := scanAnnotationSpans(content)

	var tableID string
	if tableName != "" {
		tableID = graph.TableID(tableName)
	}

	for _, matches := range fieldRe.FindAllStringSubmatch(content, -1) {
		fieldType := strings.TrimSpace(matches[1])
		fieldName := matches[2]
		fieldID := graph.FieldID(classID, fieldName)
		anns := parseAnnotations(annotationSpans, content, matches[0])

		props := map[string]any{
			"package":  pkgName,
			"name":     fieldName,
			"type":     fieldType,
			"language": string(common.LangJava),
		}

		if strings.Contains(matches[0], "static") {
			props["static"] = true
		}

		columnName := ""
		generated := false
		for _, ann := range anns {
			switch ann.Name {
			case "Autowired", "Inject":
				pending = append(pending, pendingEdge{graph.EdgeTypeInjected, classID, resolveType(fieldType, pkgName, imports)})
			case "Column":
				columnName = firstQuoted(ann.Args)
			case "GeneratedValue":
				generated = true
			}
		}

		if tableID != "" {
			if columnName == "" {
				columnName = fieldName
			}
			props["column"] = columnName
			if generated {
				props["generated"] = true
			}
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
					"language": string(common.LangJava),
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

// parseAnnotations extracts the annotations that appear on the contiguous
// run of lines immediately before a declaration.
// annotationSpan is a single `@Name(args)` annotation located in some text,
// with the byte range it occupies (args consumed via balanced-paren
// scanning, so a multi-line annotation is captured as one span).
type annotationSpan struct {
	start, end int
	ann        javaAnnotation
}

// scanAnnotationSpans finds every annotation declaration in content. Each
// annotation's optional parenthesized arguments are consumed with
// common.ScanBalanced rather than a same-line regex, so arguments spanning
// multiple lines (e.g. a multi-line `@RequestMapping(...)`) are captured
// intact.
func scanAnnotationSpans(content string) []annotationSpan {
	var spans []annotationSpan
	for _, m := range annotationNameRe.FindAllStringSubmatchIndex(content, -1) {
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

		spans = append(spans, annotationSpan{start: start, end: end, ann: javaAnnotation{Name: name, Args: args}})
	}
	return spans
}

// parseAnnotations returns the annotations, in source order, forming the
// contiguous run immediately preceding declaration — i.e. every span in
// spans whose end is separated from the next annotation's start (or from
// declaration itself) by nothing but whitespace.
func parseAnnotations(spans []annotationSpan, content, declaration string) []javaAnnotation {
	idx := strings.Index(content, declaration)
	if idx <= 0 {
		return nil
	}

	var anns []javaAnnotation
	cursor := idx
	for i := len(spans) - 1; i >= 0; i-- {
		sp := spans[i]
		if sp.end > cursor {
			continue
		}
		if strings.TrimSpace(content[sp.end:cursor]) != "" {
			break
		}
		anns = append(anns, sp.ann)
		cursor = sp.start
	}

	for i, j := 0, len(anns)-1; i < j; i, j = i+1, j-1 {
		anns[i], anns[j] = anns[j], anns[i]
	}
	return anns
}

// firstQuoted returns the first double-quoted string literal in s, e.g.
// `name = "users"` -> "users". Returns "" if none is found.
func firstQuoted(s string) string {
	m := quotedRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	return m[1]
}

// extractCalledNames does a best-effort scan of a method body for call
// expressions (`name(`, `obj.name(`, `this.name(`), skipping control-flow
// keywords that also precede "(".
func extractCalledNames(body string) []string {
	var names []string
	for _, m := range callExprRe.FindAllStringSubmatch(body, -1) {
		name := m[1]
		if javaCallKeywords[name] {
			continue
		}
		names = append(names, name)
	}
	return names
}

// methodBody returns the text between the braces of a method declaration's
// body, or "" if the declaration has no body (e.g. an abstract or interface
// method ending in ";").
func methodBody(content, declaration string) string {
	start := strings.Index(content, declaration)
	if start < 0 {
		return ""
	}

	braceIdx := -1
	if strings.HasSuffix(declaration, "{") {
		braceIdx = start + len(declaration) - 1
	} else {
		i := start + len(declaration)
		for i < len(content) && (content[i] == ' ' || content[i] == '\t' || content[i] == '\n' || content[i] == '\r') {
			i++
		}
		if i >= len(content) || content[i] != '{' {
			return ""
		}
		braceIdx = i
	}

	depth := 0
	for i := braceIdx; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[braceIdx+1 : i]
			}
		}
	}
	return content[braceIdx+1:]
}

// extractClassBody extracts the body of a class declaration.
func extractClassBody(content, classDecl string) string {
	startIdx := strings.Index(content, classDecl)
	if startIdx < 0 {
		return ""
	}

	// Find the opening brace
	braceIdx := strings.Index(content[startIdx:], "{")
	if braceIdx < 0 {
		return ""
	}

	// Find the matching closing brace
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

// extractDependencies extracts dependencies from pom.xml or build.gradle.
func (e *JavaExtractor) extractDependencies(repoPath string, g *graph.Graph) []string {
	var warnings []string

	// Check for pom.xml
	pomPath := filepath.Join(repoPath, "pom.xml")
	if _, err := os.Stat(pomPath); err == nil {
		content, readErr := os.ReadFile(pomPath)
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("pom.xml: %v", readErr))
		} else {
			extractMavenDependencies(string(content), g)
		}
	}

	// Check for build.gradle
	gradlePath := filepath.Join(repoPath, "build.gradle")
	if _, err := os.Stat(gradlePath); err == nil {
		content, readErr := os.ReadFile(gradlePath)
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("build.gradle: %v", readErr))
		} else {
			extractGradleDependencies(string(content), g)
		}
	}

	return warnings
}

// Maven dependency regex
var mavenDepRe = regexp.MustCompile(`(?s)<dependency>\s*<groupId>([^<]+)</groupId>\s*<artifactId>([^<]+)</artifactId>(?:\s*<version>[^<]*</version>)?\s*</dependency>`)

// extractMavenDependencies extracts dependencies from pom.xml.
func extractMavenDependencies(content string, g *graph.Graph) {
	for _, matches := range mavenDepRe.FindAllStringSubmatch(content, -1) {
		groupId := matches[1]
		artifactId := matches[2]

		depID := graph.ExternalDependencyID(groupId + ":" + artifactId)
		g.AddNode(&graph.Node{
			ID:   depID,
			Type: graph.NodeTypeExternalDependency,
			Properties: map[string]any{
				"group_id":    groupId,
				"artifact_id": artifactId,
				"language":    string(common.LangJava),
			},
			Hash: graph.ContentHash(groupId + ":" + artifactId),
		})
	}
}

// Gradle dependency regex
var gradleDepRe = regexp.MustCompile(`(?:implementation|api|compile)\s+['"]([^'"]+):([^'"]+)(?::([^'"]+))?['"]`)

// extractGradleDependencies extracts dependencies from build.gradle.
func extractGradleDependencies(content string, g *graph.Graph) {
	for _, matches := range gradleDepRe.FindAllStringSubmatch(content, -1) {
		groupId := matches[1]
		artifactId := matches[2]

		depID := graph.ExternalDependencyID(groupId + ":" + artifactId)
		g.AddNode(&graph.Node{
			ID:   depID,
			Type: graph.NodeTypeExternalDependency,
			Properties: map[string]any{
				"group_id":    groupId,
				"artifact_id": artifactId,
				"language":    string(common.LangJava),
			},
			Hash: graph.ContentHash(groupId + ":" + artifactId),
		})
	}
}

// resolveType resolves a type name to its full ID.
func resolveType(typeName, currentPkg string, imports []string) string {
	// Check if it's a fully qualified name
	if strings.Contains(typeName, ".") {
		return typeName
	}

	// Check imports
	for _, imp := range imports {
		if strings.HasSuffix(imp, "."+typeName) {
			return imp
		}
	}

	// Assume same package
	return currentPkg + "." + typeName
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
