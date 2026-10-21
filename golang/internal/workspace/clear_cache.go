package workspace

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ClearProjectCacheDir removes only the selected cache directory's contents.
func ClearProjectCacheDir(kind string) error {
	dir, err := projectCacheDir(kind)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

// ProjectCacheDirSize returns the size of the selected cache directory.
func ProjectCacheDirSize(kind string) (int64, error) {
	dir, err := projectCacheDir(kind)
	if err != nil {
		return 0, err
	}
	var total int64
	err = filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, walkErr error) error {
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
	if err != nil && !os.IsNotExist(err) {
		return total, err
	}
	return total, nil
}

func projectCacheDir(kind string) (string, error) {
	switch kind {
	case "casc":
		return filepath.Join(FindRepoRoot(), ".cache"), nil
	case "browse":
		return filepath.Join(FindRepoRoot(), "exported-assets-browse"), nil
	default:
		return "", fmt.Errorf("cache kind must be casc or browse")
	}
}
