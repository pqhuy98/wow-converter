package server

import (
	"sync"
	"time"
)

// Response cache with 10s TTL keyed by endpoint + stable body + buildKey.
type cacheEntry struct {
	ts     time.Time
	status int
	obj    any
}

type ResponseCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
	ttl   time.Duration
}

func NewResponseCache(ttl time.Duration) *ResponseCache {
	return &ResponseCache{items: make(map[string]cacheEntry), ttl: ttl}
}

func (c *ResponseCache) Get(key string) (int, any, bool) {
	c.mu.RLock()
	ent, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return 0, nil, false
	}
	if time.Since(ent.ts) > c.ttl {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return 0, nil, false
	}
	return ent.status, ent.obj, true
}

func (c *ResponseCache) Set(key string, status int, obj any) {
	c.mu.Lock()
	c.items[key] = cacheEntry{ts: time.Now(), status: status, obj: obj}
	c.mu.Unlock()
}
