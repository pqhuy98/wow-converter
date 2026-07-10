package texturesource

import (
	"strings"
	"sync"
)

// Kind identifies texture pixel source type.
type Kind string

const (
	KindBLP Kind = "blp"
	KindPNG Kind = "png"
)

// Source holds raw texture data for export.
type Source struct {
	Kind       Kind
	FileDataID int
	PNG        []byte
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Source{}
)

// Register registers a texture path relative to exportAssetDir.
func Register(relativePath string, source Source) {
	key := normalize(relativePath)
	registryMu.Lock()
	registry[key] = source
	registryMu.Unlock()
}

// Get returns a registered texture source.
func Get(relativePath string) (Source, bool) {
	key := normalize(relativePath)
	registryMu.RLock()
	s, ok := registry[key]
	registryMu.RUnlock()
	return s, ok
}

// Has reports whether a texture source is registered.
func Has(relativePath string) bool {
	key := normalize(relativePath)
	registryMu.RLock()
	_, ok := registry[key]
	registryMu.RUnlock()
	return ok
}

// Clear drops all registered texture sources (e.g. on CASC unload).
func Clear() {
	registryMu.Lock()
	registry = map[string]Source{}
	registryMu.Unlock()
}

// CacheStats returns converter texture registry sizes and approximate PNG bytes.
func CacheStats() (entries int, pngBytes int64) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	entries = len(registry)
	for _, src := range registry {
		pngBytes += int64(len(src.PNG))
	}
	return entries, pngBytes
}

func normalize(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
