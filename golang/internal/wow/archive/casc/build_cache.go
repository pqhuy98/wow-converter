package casc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/formats"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

var (
	cacheIntegrity    map[string]string
	integrityInitOnce sync.Once
	integrityInitErr  error
	integrityMu       sync.RWMutex
)

func ensureCacheIntegrity() (map[string]string, error) {
	integrityInitOnce.Do(func() {
		cacheIntegrity, integrityInitErr = formats.ReadJSON(constants.Cache.IntegrityFile, false)
		if integrityInitErr != nil || cacheIntegrity == nil {
			log.Write("Unable to load cache integrity file; entire cache will be invalidated.")
			if integrityInitErr != nil {
				log.Write("%s", integrityInitErr.Error())
			}
			cacheIntegrity = map[string]string{}
			integrityInitErr = nil
		}
	})
	return cacheIntegrity, integrityInitErr
}

// BuildCacheMeta holds build cache manifest metadata.
type BuildCacheMeta struct {
	LastAccess int64 `json:"lastAccess,omitempty"`
}

// BuildCache caches CDN-downloaded CASC data per build.
type BuildCache struct {
	Key          string
	Meta         BuildCacheMeta
	CacheDir     string
	ManifestPath string
}

// NewBuildCache creates a build cache for the given key.
func NewBuildCache(key string) *BuildCache {
	cacheDir := filepath.Join(constants.Cache.DirBuilds, key)
	return &BuildCache{
		Key:          key,
		CacheDir:     cacheDir,
		ManifestPath: filepath.Join(cacheDir, constants.Cache.BuildManifest),
	}
}

// Init initializes the build cache instance.
func (c *BuildCache) Init() error {
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return err
	}
	if data, err := os.ReadFile(c.ManifestPath); err == nil {
		_ = json.Unmarshal(data, &c.Meta)
	} else {
		log.Write("No cache manifest found for %s", c.Key)
	}
	c.Meta.LastAccess = time.Now().UnixMilli()
	go c.SaveManifest()
	return nil
}

// GetFile attempts to get a file from this build cache.
func (c *BuildCache) GetFile(file string, dir string) (*buffer.Buffer, error) {
	filePath := c.GetFilePath(file, dir)
	if _, err := os.Stat(filePath); err != nil {
		return nil, nil
	}
	integrity, err := ensureCacheIntegrity()
	if err != nil {
		return nil, nil
	}
	integrityMu.RLock()
	integrityHash, ok := integrity[filePath]
	integrityMu.RUnlock()
	if !ok {
		log.Write("Cache file has no integrity record, ignoring (%s)", filePath)
		return nil, nil
	}
	data, err := buffer.ReadFile(filePath)
	if err != nil {
		return nil, nil
	}
	dataHash := data.CalculateHash("sha1", "hex")
	if dataHash != integrityHash {
		log.Write("Bad integrity for file %s, rejecting cache (%s != %s)", filePath, dataHash, integrityHash)
		return nil, nil
	}
	return data, nil
}

// GetFilePath returns a direct path to a cached file.
func (c *BuildCache) GetFilePath(file string, dir string) string {
	if dir == "" {
		dir = c.CacheDir
	}
	return filepath.Join(dir, file)
}

// StoreFile stores a file in this build cache.
func (c *BuildCache) StoreFile(file string, data *buffer.Buffer, dir string) error {
	filePath := c.GetFilePath(file, dir)
	if dir != "" {
		if err := formats.CreateDirectory(filepath.Dir(filePath)); err != nil {
			return err
		}
	}
	integrity, err := ensureCacheIntegrity()
	if err != nil {
		return err
	}
	hash := data.CalculateHash("sha1", "hex")
	integrityMu.Lock()
	integrity[filePath] = hash
	integrityMu.Unlock()
	if err := os.WriteFile(filePath, data.Raw(), 0o644); err != nil {
		return err
	}
	return c.SaveCacheIntegrity()
}

// SaveCacheIntegrity saves cache integrity to disk.
func (c *BuildCache) SaveCacheIntegrity() error {
	integrityMu.Lock()
	snapshot := make(map[string]string, len(cacheIntegrity))
	for k, v := range cacheIntegrity {
		snapshot[k] = v
	}
	integrityMu.Unlock()
	if err := formats.CreateDirectory(filepath.Dir(constants.Cache.IntegrityFile)); err != nil {
		return err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return os.WriteFile(constants.Cache.IntegrityFile, data, 0o644)
}

// SaveManifest saves the manifest for this build cache.
func (c *BuildCache) SaveManifest() error {
	data, err := json.Marshal(c.Meta)
	if err != nil {
		return err
	}
	return os.WriteFile(c.ManifestPath, data, 0o644)
}
