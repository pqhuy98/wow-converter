// Package client provides shared on-disk cache helpers for raw WoW files.
package client

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

// rawBuildCacheTTL is how long a build's raw files stay after they were last loaded or read.
// Switching retail and classic inside this window keeps both caches.
const rawBuildCacheTTL = 14 * 24 * time.Hour

// rawBuildTouchGap limits how often a busy build rewrites its directory mtime.
const rawBuildTouchGap = time.Hour

var rawDataDir = filepath.Join(constants.DataPath, "data")

var (
	rawBuildTouchMu sync.Mutex
	rawBuildTouchAt = map[string]time.Time{}
)

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
	noteRawBuildUsed(buildKey)
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
	noteRawBuildUsed(buildKey)
	return nil
}

// ExpireOtherRawBuildCaches drops raw-file caches that have not been loaded or read
// within rawBuildCacheTTL. activeBuildKey is marked used first, so the build just
// loaded is kept. The shared CASC archive cache is not under this directory.
// An empty key deletes nothing. A missing cache directory is not an error.
func ExpireOtherRawBuildCaches(activeBuildKey string) (int, error) {
	n, err := expireOtherRawBuildCaches(rawDataDir, activeBuildKey, time.Now())
	if err != nil {
		log.Write("Failed to expire old raw build caches: %v", err)
		return n, err
	}
	if n > 0 {
		days := int(rawBuildCacheTTL.Hours() / 24)
		log.Write("Expired %d raw build cache(s) last used more than %d days ago", n, days)
	}
	return n, nil
}

func expireOtherRawBuildCaches(dir, activeBuildKey string, now time.Time) (int, error) {
	if activeBuildKey == "" {
		return 0, nil
	}
	activeDir := filepath.Join(dir, activeBuildKey)
	if err := os.MkdirAll(activeDir, 0o755); err != nil {
		return 0, err
	}
	if err := os.Chtimes(activeDir, now, now); err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	var first error
	for _, ent := range entries {
		if !ent.IsDir() || ent.Name() == activeBuildKey {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		if now.Sub(info.ModTime()) < rawBuildCacheTTL {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, ent.Name())); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		removed++
	}
	return removed, first
}

func noteRawBuildUsed(buildKey string) {
	if buildKey == "" {
		return
	}
	now := time.Now()
	rawBuildTouchMu.Lock()
	if prev, ok := rawBuildTouchAt[buildKey]; ok && now.Sub(prev) < rawBuildTouchGap {
		rawBuildTouchMu.Unlock()
		return
	}
	rawBuildTouchAt[buildKey] = now
	rawBuildTouchMu.Unlock()
	_ = os.Chtimes(filepath.Join(rawDataDir, buildKey), now, now)
}
