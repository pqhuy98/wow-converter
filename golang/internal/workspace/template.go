package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveTemplateEmptyDir returns the WC3 empty-map template directory under resources/.
func ResolveTemplateEmptyDir() (string, error) {
	dir := filepath.Join(FindRepoRoot(), "resources", "template-empty.w3x")
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir, nil
	}
	return "", fmt.Errorf("missing WC3 map template (template-empty.w3x): %s", dir)
}
