package server

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// minimal in-memory config with optional persistence for early endpoints
type ConfigStore struct {
	mu     sync.RWMutex
	values map[string]any
	path   string
}

func NewConfigStore() *ConfigStore {
	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".wowexportd", "config.json")
	return &ConfigStore{values: map[string]any{
		// defaults mirroring wow.export expectations where relevant
		"exportDirectory":     defaultExportDir(),
		"listfileURL":         "https://github.com/wowdev/wow-listfile/releases/latest/download/community-listfile.csv",
		"listfileFallbackURL": "https://github.com/wowdev/wow-listfile/releases/latest/download/community-listfile.csv",
		// days; 0 disables refresh (match wow.export semantics)
		"listfileCacheRefresh": 7,
	}, path: cfgPath}
}

func defaultExportDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wowexportd", "export")
}

func (c *ConfigStore) GetAll() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]any, len(c.values))
	for k, v := range c.values {
		out[k] = v
	}
	return out
}

func (c *ConfigStore) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.values[key]
	return v, ok
}

func (c *ConfigStore) Set(key string, value any) error {
	if key == "" {
		return errors.New("empty key")
	}
	c.mu.Lock()
	c.values[key] = value
	c.mu.Unlock()
	return nil
}
