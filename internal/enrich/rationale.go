package enrich

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"kgraph/internal/graph"
)

// rationaleTagRe matches a single-line comment body tagged with one of the
// recognized rationale prefixes, per rationale-extraction's "Rationale
// comments SHALL be extracted as nodes" requirement.
var rationaleTagRe = regexp.MustCompile(`^(NOTE|WHY|HACK|TODO|FIXME|WARNING):\s*(.*)$`)

// lineCommentPrefix returns the single-line comment marker used by the
// language inferred from filePath's extension.
func lineCommentPrefix(filePath string) string {
	if strings.ToLower(filepath.Ext(filePath)) == ".py" {
		return "#"
	}
	return "//"
}

// ExtractRationale scans content for comments tagged NOTE, WHY, HACK, TODO,
// FIXME, or WARNING and returns a Rationale node for each, keyed by
// deterministic ID so rebuilds don't duplicate them. A tag's text may
// continue onto immediately-following comment lines sharing the same
// prefix, per rationale-extraction's "Rationale comment text SHALL span
// continuation lines" requirement — continuation stops at a blank line, a
// non-comment line, end of file, or a line that is itself a fresh tag, so
// two adjacent tags (e.g. a NOTE followed by a WHY) stay separate nodes. g
// is unused today but kept in the signature for symmetry with
// ExtractDocstrings and to allow future de-duplication against existing
// nodes.
func ExtractRationale(g *graph.Graph, filePath, content string) []*graph.Node {
	_ = g
	prefix := lineCommentPrefix(filePath)
	lines := strings.Split(content, "\n")

	var nodes []*graph.Node
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		m := rationaleTagRe.FindStringSubmatch(body)
		if m == nil {
			continue
		}
		kind, text := m[1], strings.TrimSpace(m[2])
		if text == "" {
			continue
		}

		textParts := []string{text}
		end := i
		for j := i + 1; j < len(lines); j++ {
			ct := strings.TrimSpace(lines[j])
			if !strings.HasPrefix(ct, prefix) {
				break
			}
			cbody := strings.TrimSpace(strings.TrimPrefix(ct, prefix))
			if cbody == "" || rationaleTagRe.MatchString(cbody) {
				break
			}
			textParts = append(textParts, cbody)
			end = j
		}

		lineStart := i + 1
		nodes = append(nodes, rationaleNode(filePath, lineStart, end+1, kind, strings.Join(textParts, " ")))
		i = end
	}
	return nodes
}

func rationaleNode(filePath string, lineStart, lineEnd int, kind, text string) *graph.Node {
	return &graph.Node{
		ID:        graph.RationaleID(filePath, strconv.Itoa(lineStart), kind),
		Type:      graph.NodeTypeRationale,
		File:      filePath,
		LineStart: lineStart,
		LineEnd:   lineEnd,
		Signature: kind + ": " + text,
		Hash:      graph.ContentHash(kind + text),
		Properties: map[string]any{
			"kind": kind,
			"text": text,
		},
	}
}

// ExtractDocstrings scans content for module/class/function-level
// documentation comments (Go doc comments, Python triple-quoted
// docstrings, JSDoc blocks) and returns a Rationale node with kind
// "DOCSTRING" for each, per rationale-extraction's docstring requirement.
// The language is inferred from filePath's extension.
func ExtractDocstrings(g *graph.Graph, filePath, content string) []*graph.Node {
	_ = g
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".go":
		return extractGoDocstrings(filePath, content)
	case ".py":
		return extractPythonDocstrings(filePath, content)
	case ".ts", ".tsx", ".js", ".jsx", ".java":
		return extractBlockDocstrings(filePath, content)
	default:
		return nil
	}
}

var goDeclRe = regexp.MustCompile(`^(func|type|var|const)\s`)

// extractGoDocstrings treats a contiguous run of "//" line comments
// immediately preceding a top-level func/type/var/const declaration as
// that declaration's doc comment.
func extractGoDocstrings(filePath, content string) []*graph.Node {
	lines := strings.Split(content, "\n")
	var nodes []*graph.Node
	for i := range lines {
		if !goDeclRe.MatchString(lines[i]) {
			continue
		}
		start := i - 1
		var commentLines []string
		for start >= 0 && strings.HasPrefix(strings.TrimSpace(lines[start]), "//") {
			text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[start]), "//"))
			commentLines = append([]string{text}, commentLines...)
			start--
		}
		if len(commentLines) == 0 {
			continue
		}
		// A comment block that's just a single rationale-tagged line (e.g.
		// "// NOTE: ...") is already captured by ExtractRationale — don't
		// double-count it as a docstring.
		if len(commentLines) == 1 && rationaleTagRe.MatchString(commentLines[0]) {
			continue
		}
		text := strings.TrimSpace(strings.Join(commentLines, " "))
		if text == "" {
			continue
		}
		lineStart := start + 2
		lineEnd := i
		nodes = append(nodes, rationaleNode(filePath, lineStart, lineEnd, "DOCSTRING", text))
	}
	return nodes
}

var pyDeclRe = regexp.MustCompile(`^\s*(def|class)\s+\w+`)

// extractPythonDocstrings treats a triple-quoted string literal as the
// first non-blank line inside a def/class body as that entity's docstring.
func extractPythonDocstrings(filePath, content string) []*graph.Node {
	lines := strings.Split(content, "\n")
	var nodes []*graph.Node
	for i := range lines {
		if !pyDeclRe.MatchString(lines[i]) {
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= len(lines) {
			continue
		}
		trimmed := strings.TrimSpace(lines[j])
		var quote string
		switch {
		case strings.HasPrefix(trimmed, `"""`):
			quote = `"""`
		case strings.HasPrefix(trimmed, `'''`):
			quote = `'''`
		default:
			continue
		}

		rest := trimmed[len(quote):]
		var textLines []string
		endLine := j
		if idx := strings.Index(rest, quote); idx >= 0 {
			textLines = append(textLines, rest[:idx])
		} else {
			textLines = append(textLines, rest)
			k := j + 1
			for k < len(lines) {
				if idx := strings.Index(lines[k], quote); idx >= 0 {
					textLines = append(textLines, lines[k][:idx])
					endLine = k
					break
				}
				textLines = append(textLines, lines[k])
				endLine = k
				k++
			}
		}
		text := strings.TrimSpace(strings.Join(textLines, " "))
		if text == "" {
			continue
		}
		nodes = append(nodes, rationaleNode(filePath, j+1, endLine+1, "DOCSTRING", text))
	}
	return nodes
}

var blockDeclRe = regexp.MustCompile(`^\s*(export\s+)?(default\s+)?(public\s+|private\s+|protected\s+)?(abstract\s+)?(async\s+)?(function|class|const|interface|type|enum)\s`)

// extractBlockDocstrings treats a "/** ... */" comment block immediately
// preceding a declaration line as JSDoc/Javadoc documentation.
func extractBlockDocstrings(filePath, content string) []*graph.Node {
	lines := strings.Split(content, "\n")
	var nodes []*graph.Node
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "/**") {
			continue
		}
		start := i
		var textLines []string
		first := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "/**"), "*/"))
		if first != "" {
			textLines = append(textLines, first)
		}
		end := i
		if !strings.Contains(trimmed, "*/") {
			j := i + 1
			for j < len(lines) {
				l := strings.TrimSpace(lines[j])
				closed := strings.Contains(l, "*/")
				if closed {
					l = strings.TrimSuffix(l, "*/")
				}
				l = strings.TrimSpace(strings.TrimPrefix(l, "*"))
				if l != "" {
					textLines = append(textLines, l)
				}
				if closed {
					end = j
					break
				}
				end = j
				j++
			}
			i = end
		}

		k := end + 1
		for k < len(lines) && strings.TrimSpace(lines[k]) == "" {
			k++
		}
		if k >= len(lines) || !blockDeclRe.MatchString(lines[k]) {
			continue
		}
		text := strings.TrimSpace(strings.Join(textLines, " "))
		if text == "" {
			continue
		}
		nodes = append(nodes, rationaleNode(filePath, start+1, end+1, "DOCSTRING", text))
	}
	return nodes
}

// LinkRationale creates an `explains` edge from each rationale node to
// target, per rationale-extraction's requirement that a Rationale node
// connect to the nearest code entity it documents. Confidence is INFERRED:
// the association is a line-proximity heuristic, not something read
// directly off the AST.
func LinkRationale(g *graph.Graph, rationale []*graph.Node, target *graph.Node) {
	if target == nil {
		return
	}
	for _, r := range rationale {
		if r == nil {
			continue
		}
		_ = g.AddEdge(&graph.Edge{
			ID:         graph.EdgeID(graph.EdgeTypeExplains, r.ID, target.ID),
			Type:       graph.EdgeTypeExplains,
			SrcID:      r.ID,
			DstID:      target.ID,
			Confidence: graph.ConfidenceInferred,
		})
	}
}
