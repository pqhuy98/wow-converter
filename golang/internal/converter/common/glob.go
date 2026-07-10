package common

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// GlobExportFiles finds files under root matching a slash-separated glob pattern.
// Supports ** and * wildcards (e.g. "**/northrend/adt_21_27.obj").
func GlobExportFiles(root, pattern string) ([]string, error) {
	pattern = filepath.ToSlash(strings.TrimPrefix(pattern, "./"))
	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if matchGlobPattern(pattern, rel) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func matchGlobPattern(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)
	if pattern == path {
		return true
	}
	pParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	return matchGlobParts(pParts, pathParts)
}

func matchGlobParts(pattern, path []string) bool {
	for len(pattern) > 0 {
		p := pattern[0]
		pattern = pattern[1:]
		if p == "**" {
			if len(pattern) == 0 {
				return true
			}
			for i := 0; i <= len(path); i++ {
				if matchGlobParts(pattern, path[i:]) {
					return true
				}
			}
			return false
		}
		if len(path) == 0 {
			return false
		}
		if !matchGlobSegment(p, path[0]) {
			return false
		}
		path = path[1:]
	}
	return len(path) == 0
}

func matchGlobSegment(pattern, segment string) bool {
	if pattern == "*" {
		return true
	}
	pRunes := []rune(pattern)
	sRunes := []rune(segment)
	return matchGlobChars(pRunes, sRunes)
}

func matchGlobChars(p, s []rune) bool {
	for len(p) > 0 {
		if p[0] == '*' {
			for i := 0; i <= len(s); i++ {
				if matchGlobChars(p[1:], s[i:]) {
					return true
				}
			}
			return false
		}
		if len(s) == 0 {
			return false
		}
		if p[0] != s[0] {
			return false
		}
		p = p[1:]
		s = s[1:]
	}
	return len(s) == 0
}
