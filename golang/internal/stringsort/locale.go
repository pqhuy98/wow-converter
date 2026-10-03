package stringsort

import (
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
	NewEnglishCollator().SortStrings(ss)
}

// SortBy sorts items in-place by key using English locale rules.
// Safe for concurrent calls on different slices.
func SortBy[T any](items []T, key func(T) string) {
	if len(items) <= 1 {
		return
	}
	l := &keyedLister[T]{items: items, key: key}
	NewEnglishCollator().Sort(l)
}

type keyedLister[T any] struct {
	items []T
	key   func(T) string
}

func (l *keyedLister[T]) Len() int { return len(l.items) }

func (l *keyedLister[T]) Swap(i, j int) {
	l.items[i], l.items[j] = l.items[j], l.items[i]
}

func (l *keyedLister[T]) Bytes(i int) []byte {
	return []byte(l.key(l.items[i]))
}
