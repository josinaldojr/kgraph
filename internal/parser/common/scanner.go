package common

// ScanBalanced scans s forward from start (which must index the opening
// delimiter open) to the matching close delimiter, honoring nesting depth
// and skipping over delimiters that appear inside single- or double-quoted
// string literals. It operates on the whole remaining text rather than a
// single line, so multi-line constructs (a decorator or annotation whose
// arguments span several lines) are captured correctly.
//
// content is the text between the opening and closing delimiter (exclusive
// of both). end is the index one past the matching close delimiter. ok is
// false if s[start] is not open, or no matching close is found (unbalanced
// input) — in the latter case content/end are zero-valued.
func ScanBalanced(s string, start int, open, close byte) (content string, end int, ok bool) {
	if start < 0 || start >= len(s) || s[start] != open {
		return "", 0, false
	}

	depth := 0
	var quote byte
	for i := start; i < len(s); i++ {
		c := s[i]

		if quote != 0 {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}

		switch {
		case c == '\'' || c == '"':
			quote = c
		case c == open:
			depth++
		case c == close:
			depth--
			if depth == 0 {
				return s[start+1 : i], i + 1, true
			}
		}
	}

	return "", 0, false
}
