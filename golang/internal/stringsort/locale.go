package stringsort

import (
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

var englishCollator = collate.New(language.English)

// Less matches JavaScript Array.sort with localeCompare for browse/list ordering.
// The global collator is not safe for concurrent use; use NewEnglishCollator for parallel sorts.
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
