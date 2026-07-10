package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
)

// ClearProjectCacheDir removes all contents of the repo `.cache` directory.
func ClearProjectCacheDir() error {
	cacheDir := filepath.Join(FindRepoRoot(), ".cache")
	if err := os.RemoveAll(cacheDir); err != nil {
		return err
	}
	return os.MkdirAll(cacheDir, 0o755)
}

// ProjectCacheDirSize returns the total byte size of files under the repo `.cache` directory.
func ProjectCacheDirSize() (int64, error) {
	cacheDir := filepath.Join(FindRepoRoot(), ".cache")
	var total int64
	err := filepath.WalkDir(cacheDir, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return total, err
}
