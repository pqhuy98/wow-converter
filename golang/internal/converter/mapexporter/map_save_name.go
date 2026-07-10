package mapexporter

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	mapSaveNameInvalidChars = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)
	mapSaveNameMultiUnderscore = regexp.MustCompile(`_+`)
	mapSaveNameTrimEdges = regexp.MustCompile(`^[_.-]+|[_.-]+$`)
	mapSaveNameW3xSuffix = regexp.MustCompile(`(?i)\.w3x$`)
)

func stripMapSaveNameExtension(name string) string {
	base := strings.TrimSpace(name)
	for mapSaveNameW3xSuffix.MatchString(base) {
		base = mapSaveNameW3xSuffix.ReplaceAllString(base, "")
	}
	return base
}

func sanitizeMapSaveNameBase(name string) string {
	base := stripMapSaveNameExtension(name)
	base = mapSaveNameInvalidChars.ReplaceAllString(base, "_")
	base = mapSaveNameMultiUnderscore.ReplaceAllString(base, "_")
	base = mapSaveNameTrimEdges.ReplaceAllString(base, "")
	return base
}

func NormalizeMapSaveName(name string) string {
	base := filepath.Base(sanitizeMapSaveNameBase(name))
	if base == "" || base == "." || base == ".." {
		return ""
	}
	return base + ".w3x"
}
