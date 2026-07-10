// Package client provides shared on-disk cache helpers for raw WoW files.
package client

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pqhuy98/wow-converter/internal/wow/constants"
)

var rawDataDir = filepath.Join(constants.DataPath, "data")

// RawFileCachePath returns the cache path for a raw file.
func RawFileCachePath(buildKey string, fileDataID int) string {
	return filepath.Join(rawDataDir, buildKey, strconv.Itoa(fileDataID))
}

// ReadRawCachedFile reads a cached raw file, or nil when absent.
func ReadRawCachedFile(buildKey string, fileDataID int) ([]byte, error) {
	data, err := os.ReadFile(RawFileCachePath(buildKey, fileDataID))
	if err != nil {
		return nil, nil
	}
	if len(data) == 0 {
		return nil, nil
	}
	return data, nil
}

// RawCachedFileExistsSync reports whether a cached raw file exists.
func RawCachedFileExistsSync(buildKey string, fileDataID int) bool {
	_, err := os.Stat(RawFileCachePath(buildKey, fileDataID))
	return err == nil
}

// WriteRawCachedFile atomically persists a raw file.
func WriteRawCachedFile(buildKey string, fileDataID int, data []byte) error {
	dest := RawFileCachePath(buildKey, fileDataID)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmpBytes := make([]byte, 6)
	if _, err := rand.Read(tmpBytes); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.%s.tmp", dest, hex.EncodeToString(tmpBytes))
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		if _, statErr := os.Stat(dest); statErr != nil {
			return err
		}
	}
	return nil
}
