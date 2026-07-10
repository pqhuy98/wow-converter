package rest

import "strings"

const maxRegexPatternLen = 256

// SafeRegexPattern rejects oversized or obviously catastrophic patterns before compile.
func SafeRegexPattern(pattern string) (string, bool) {
	if pattern == "" || len(pattern) > maxRegexPatternLen {
		return "", false
	}
	// ponytail: heuristic only; Go regexp is RE2 (linear), but cap pattern size for parity with TS.
	if strings.Contains(pattern, "(?") && strings.ContainsAny(pattern, "+*{") {
		return "", false
	}
	if nestedQuantifierPattern(pattern) {
		return "", false
	}
	return pattern, true
}

func nestedQuantifierPattern(pattern string) bool {
	for i := 0; i < len(pattern)-2; i++ {
		if pattern[i] != '(' {
			continue
		}
		depth := 1
		for j := i + 1; j < len(pattern); j++ {
			switch pattern[j] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					inner := pattern[i+1 : j]
					if strings.ContainsAny(inner, "+*") && j+1 < len(pattern) && strings.ContainsAny(pattern[j+1:], "+*{") {
						return true
					}
					break
				}
			}
		}
	}
	return false
}
