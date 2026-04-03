package transpiler

import "strings"

// findBalancedParen scans from position start in s, counting parenthesis depth,
// and returns the index of the matching closing paren. Returns -1 if not found.
func findBalancedParen(s string, start int) int {
	depth := 1
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitTopLevelCSV splits a comma-separated argument string while respecting
// nested (), {}, and [] groupings plus quoted string contents.
func splitTopLevelCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var result []string
	parenDepth := 0
	braceDepth := 0
	bracketDepth := 0
	start := 0
	var quote byte
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}

		switch ch {
		case '\'', '"':
			quote = ch
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case ',':
			if parenDepth == 0 && braceDepth == 0 && bracketDepth == 0 {
				part := strings.TrimSpace(s[start:i])
				if part != "" {
					result = append(result, part)
				}
				start = i + 1
			}
		}
	}

	if part := strings.TrimSpace(s[start:]); part != "" {
		result = append(result, part)
	}
	return result
}

// splitArgs is kept as a compatibility shim for older tests and callers.
func splitArgs(s string) []string {
	return splitTopLevelCSV(s)
}
