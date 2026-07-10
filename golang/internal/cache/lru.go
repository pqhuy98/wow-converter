package cache

import "sync"

// LRU is a fixed-capacity least-recently-used string cache.
type LRU struct {
	mu      sync.Mutex
	max     int
	order   []string
	entries map[string]string
}

// NewLRU creates an LRU cache holding at most max entries.
func NewLRU(max int) *LRU {
	if max < 1 {
		max = 1
	}
	return &LRU{
		max:     max,
		entries: make(map[string]string, max),
	}
}

// Get returns a cached value and whether it was present.
func (c *LRU) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	val, ok := c.entries[key]
	if !ok {
		return "", false
	}
	c.touchLocked(key)
	return val, true
}

// Set stores a value, evicting the oldest entry when at capacity.
func (c *LRU) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[key]; ok {
		c.entries[key] = value
		c.touchLocked(key)
		return
	}
	if len(c.entries) >= c.max {
		c.evictOldestLocked()
	}
	c.entries[key] = value
	c.order = append(c.order, key)
}

func (c *LRU) touchLocked(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(append(c.order[:i:i], c.order[i+1:]...), key)
			return
		}
	}
}

func (c *LRU) evictOldestLocked() {
	if len(c.order) == 0 {
		return
	}
	oldest := c.order[0]
	c.order = c.order[1:]
	delete(c.entries, oldest)
}

// Len returns the number of cached entries.
func (c *LRU) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
