package rest

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type jsonResponse struct {
	status int
	body   any
}

func sendJSON(w http.ResponseWriter, status int, obj any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(obj)
}

type responseCacheEntry struct {
	ts     time.Time
	status int
	body   any
}

type responseCache struct {
	mu         sync.Mutex
	entries    map[string]responseCacheEntry
	ttl        time.Duration
	maxEntries int
}

func newResponseCache() *responseCache {
	return &responseCache{
		entries:    make(map[string]responseCacheEntry),
		ttl:        10 * time.Second,
		maxEntries: 128,
	}
}

func (c *responseCache) get(key string) (responseCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return responseCacheEntry{}, false
	}
	if time.Since(entry.ts) > c.ttl {
		delete(c.entries, key)
		return responseCacheEntry{}, false
	}
	return entry, true
}

func (c *responseCache) set(key string, status int, body any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.maxEntries {
		c.evictOldestLocked()
	}
	c.entries[key] = responseCacheEntry{ts: time.Now(), status: status, body: body}
}

func (c *responseCache) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	for key, entry := range c.entries {
		if oldestKey == "" || entry.ts.Before(oldest) {
			oldestKey = key
			oldest = entry.ts
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

func (c *responseCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]responseCacheEntry)
}

func (c *responseCache) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func (c *responseCache) pruneLocked() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.Sub(entry.ts) > c.ttl {
			delete(c.entries, key)
		}
	}
}

func (c *responseCache) sendAndCache(w http.ResponseWriter, key string, status int, obj any) {
	if status >= 200 && status < 300 {
		c.set(key, status, obj)
	}
	sendJSON(w, status, obj)
}

func stableCacheKey(endpoint string, body any) string {
	b, _ := json.Marshal(body)
	return endpoint + ":" + string(b)
}

func contentTypeForExt(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".json":
		return "application/json; charset=utf-8"
	case ".obj", ".mtl", ".csv":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
