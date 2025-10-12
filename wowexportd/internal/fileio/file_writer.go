package fileio

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func WriteFileAtomic(path string, data []byte, overwrite bool) error {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, fs.FileMode(0o644)); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		// cleanup best-effort
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func WriteJSONAtomic(path string, v any, overwrite bool) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, b, overwrite)
}

func JoinExportPath(baseDir, rel string) (string, error) {
	if baseDir == "" {
		return "", errors.New("missing exportDirectory")
	}
	clean := filepath.Clean(rel)
	return filepath.Join(baseDir, clean), nil
}
