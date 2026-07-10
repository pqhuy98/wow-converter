package workspace

import (
	"os"
	"path/filepath"
)

// BundledAppRoot returns the directory containing the running executable.
func BundledAppRoot() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// IsBundledLayout reports whether the exe sits in a desktop bundle (webui/out beside it).
// Matches the Bun app: no env var or launcher script required.
func IsBundledLayout() bool {
	if v := os.Getenv("WOW_CONVERTER_BUNDLED"); v == "1" || v == "true" {
		return true
	}
	_, err := os.Stat(filepath.Join(BundledAppRoot(), "webui", "out", "index.html"))
	return err == nil
}
