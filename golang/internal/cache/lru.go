package cache

import (
	"container/list"
	"sync"
)

type lruEntry struct {
	key string
	val string
}

// LRU is a fixed-capacity least-recently-used string cache.
type LRU struct {
	mu      sync.Mutex
	max     int
	order   *list.List
	entries map[string]*list.Element
}

// NewLRU creates an LRU cache holding at most max entries.
func NewLRU(max int) *LRU {
	if max < 1 {
		max = 1
	}
	return &LRU{
		max:     max,
		order:   list.New(),
		entries: make(map[string]*list.Element, max),
	}
}

// Get returns a cached value and whether it was present.
func (c *LRU) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return "", false
	}
	c.order.MoveToBack(el)
	return el.Value.(*lruEntry).val, true
}

// Set stores a value, evicting the oldest entry when at capacity.
func (c *LRU) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[key]; ok {
		el.Value.(*lruEntry).val = value
		c.order.MoveToBack(el)
		return
	}
	if c.order.Len() >= c.max {
		oldest := c.order.Front()
		if oldest != nil {
			c.order.Remove(oldest)
			e := oldest.Value.(*lruEntry)
			delete(c.entries, e.key)
		}
	}
	el := c.order.PushBack(&lruEntry{key: key, val: value})
	c.entries[key] = el
}

// Len returns the number of cached entries.
func (c *LRU) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
