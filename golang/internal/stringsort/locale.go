package stringsort

import (
	"bytes"
	"sort"
	"sync"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

var (
	englishCollator   = collate.New(language.English)
	englishCollatorMu sync.Mutex
)

// Less matches JavaScript Array.sort with localeCompare for a single comparison.
// Sort and SortBy are the path for a whole slice: each call owns a collator and compares keys.
// CompareString mutates the collator, so this lock covers the remaining one-off callers.
func Less(a, b string) bool {
	if a == b {
		return false
	}
	if a == "" {
		return true
	}
	if b == "" {
		return false
	}
	englishCollatorMu.Lock()
	defer englishCollatorMu.Unlock()
	return englishCollator.CompareString(a, b) < 0
}

// NewEnglishCollator returns a collator for locale-aware sorting in a single goroutine.
func NewEnglishCollator() *collate.Collator {
	return collate.New(language.English)
}

// Sort sorts strings using English locale rules. Safe for concurrent calls on different slices.
func Sort(ss []string) {
	if len(ss) <= 1 {
		return
	}
	SortBy(ss, func(s string) string { return s })
}

// SortBy sorts items in-place by key using English locale rules.
// Safe for concurrent calls on different slices.
func SortBy[T any](items []T, key func(T) string) {
	if len(items) <= 1 {
		return
	}
	l := &keyedLister[T]{items: items, keys: make([][]byte, len(items))}
	c := NewEnglishCollator()
	var buffer collate.Buffer
	var arena []byte
	chunkSize := min(256*1024, len(items)*256)
	for i, item := range items {
		buffer.Reset()
		k := c.KeyFromString(&buffer, key(item))
		// Collator.Sort grows one buffer while earlier keys keep its old
		// allocations alive. Fixed chunks retain only the actual key bytes.
		if len(arena) < len(k) {
			arena = make([]byte, max(chunkSize, len(k)))
		}
		copy(arena, k)
		l.keys[i] = arena[:len(k):len(k)]
		arena = arena[len(k):]
	}
	sort.Sort(l)
}

type keyedLister[T any] struct {
	items []T
	keys  [][]byte
}

func (l *keyedLister[T]) Len() int { return len(l.items) }

func (l *keyedLister[T]) Swap(i, j int) {
	l.items[i], l.items[j] = l.items[j], l.items[i]
	l.keys[i], l.keys[j] = l.keys[j], l.keys[i]
}

func (l *keyedLister[T]) Less(i, j int) bool {
	return bytes.Compare(l.keys[i], l.keys[j]) < 0
}
