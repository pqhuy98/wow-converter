package casc

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type BuildCache struct {
	key       string
	integrity map[string]string // relative path -> sha1
	manifest  map[string]any    // lastAccess, etc.
	mu        sync.Mutex
}

func NewBuildCache(key string) *BuildCache { return &BuildCache{key: key} }

func (c *BuildCache) Init() error {
	// Ensure per-build directory exists
	dir := filepath.Join(DirBuilds(), c.key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Load integrity
	c.loadIntegrity()
	// Load manifest
	c.loadManifest()
	// Update last access
	c.mu.Lock()
	if c.manifest == nil {
		c.manifest = make(map[string]any)
	}
	c.manifest["lastAccess"] = time.Now().UnixMilli()
	_ = c.saveManifestLocked()
	c.mu.Unlock()
	return nil
}

func (c *BuildCache) filePath(name string, subdir string) string {
	dir := filepath.Join(DirBuilds(), c.key)
	if subdir != "" {
		dir = filepath.Join(dir, subdir)
	}
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, name)
}

func (c *BuildCache) integrityPath() string {
	return filepath.Join(DirBuilds(), c.key, "integrity.json")
}
func (c *BuildCache) manifestPath() string { return filepath.Join(DirBuilds(), c.key, "manifest.json") }

func (c *BuildCache) loadIntegrity() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.integrity = make(map[string]string)
	if b, err := os.ReadFile(c.integrityPath()); err == nil {
		_ = json.Unmarshal(b, &c.integrity)
	}
}

func (c *BuildCache) saveIntegrityLocked() error {
	b, _ := json.Marshal(c.integrity)
	return os.WriteFile(c.integrityPath(), b, 0o644)
}

func (c *BuildCache) loadManifest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.manifest = make(map[string]any)
	if b, err := os.ReadFile(c.manifestPath()); err == nil {
		_ = json.Unmarshal(b, &c.manifest)
	}
}

func (c *BuildCache) saveManifestLocked() error {
	b, _ := json.Marshal(c.manifest)
	return os.WriteFile(c.manifestPath(), b, 0o644)
}

func (c *BuildCache) GetFile(name string, subdir string) ([]byte, error) {
	path := c.filePath(name, subdir)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	// Verify integrity if present
	rel := name
	if subdir != "" {
		rel = filepath.Join(subdir, name)
	}
	c.mu.Lock()
	if want, ok := c.integrity[rel]; ok {
		sum := sha1.Sum(b)
		got := hex.EncodeToString(sum[:])
		if got != want {
			c.mu.Unlock()
			return nil, nil
		}
	}
	// Update lastAccess
	if c.manifest == nil {
		c.manifest = make(map[string]any)
	}
	c.manifest["lastAccess"] = time.Now().UnixMilli()
	_ = c.saveManifestLocked()
	c.mu.Unlock()
	return b, nil
}

func (c *BuildCache) StoreFile(name string, data []byte, subdir string) error {
	path := c.filePath(name, subdir)
	if err := os.WriteFile(path, data, fs.FileMode(0o644)); err != nil {
		return err
	}
	// Record integrity
	rel := name
	if subdir != "" {
		rel = filepath.Join(subdir, name)
	}
	sum := sha1.Sum(data)
	c.mu.Lock()
	if c.integrity == nil {
		c.integrity = make(map[string]string)
	}
	c.integrity[rel] = hex.EncodeToString(sum[:])
	_ = c.saveIntegrityLocked()
	// Update lastAccess
	if c.manifest == nil {
		c.manifest = make(map[string]any)
	}
	c.manifest["lastAccess"] = time.Now().UnixMilli()
	_ = c.saveManifestLocked()
	c.mu.Unlock()
	return nil
}
