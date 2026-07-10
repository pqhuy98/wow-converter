package common

import (
	"os"
	"path/filepath"
	"strings"
)

// ExportAssetExists reports whether a file exists under the export asset tree.
func ExportAssetExists(absPath string) bool {
	_, err := os.Stat(filepath.Clean(absPath))
	return err == nil
}

// StripModelReferenceExt removes .m2/.wmo/.obj and .phys.* suffixes.
func StripModelReferenceExt(ref string) string {
	ref = strings.ReplaceAll(ref, "\\", "/")
	for _, ext := range []string{".m2", ".wmo", ".obj"} {
		if strings.HasSuffix(strings.ToLower(ref), ext) {
			return ref[:len(ref)-len(ext)]
		}
	}
	if idx := strings.Index(ref, ".phys."); idx >= 0 {
		return ref[:idx]
	}
	return ref
}

// NormalizeLocalModelRef returns listfile-style path without model extension.
func NormalizeLocalModelRef(ref string) string {
	return StripModelReferenceExt(ref)
}

// CachePathForLocalRef builds virtual cache path under exportAssetDir.
func CachePathForLocalRef(exportAssetDir, ref, ext string) string {
	return filepath.Join(exportAssetDir, NormalizeLocalModelRef(ref)+ext)
}
