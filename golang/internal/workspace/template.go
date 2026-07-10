package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveTemplateEmptyDir returns the WC3 empty-map template directory.
// Bundled desktop builds ship it under resources/; dev and shared hosting use maps/.
func ResolveTemplateEmptyDir() (string, error) {
	for _, c := range templateEmptyCandidates() {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("missing WC3 map template (template-empty.w3x); checked %v", templateEmptyCandidates())
}

func templateEmptyCandidates() []string {
	return []string{
		filepath.Join(BundledAppRoot(), "resources", "template-empty.w3x"),
		ResolveRepoPath("maps/template-empty.w3x"),
	}
}
