package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendAndCacheOnlyStoresSuccessfulResponses(t *testing.T) {
	cache := newResponseCache()
	key := "export:test"

	rec := httptest.NewRecorder()
	cache.sendAndCache(rec, key, http.StatusInternalServerError, map[string]any{"id": "ERR_INTERNAL"})
	if _, ok := cache.get(key); ok {
		t.Fatal("expected 500 response not to be cached")
	}

	rec = httptest.NewRecorder()
	cache.sendAndCache(rec, key, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_PARAMETERS"})
	if _, ok := cache.get(key); ok {
		t.Fatal("expected 400 response not to be cached")
	}

	rec = httptest.NewRecorder()
	cache.sendAndCache(rec, key, http.StatusOK, map[string]any{"id": "EXPORT_RESULT"})
	entry, ok := cache.get(key)
	if !ok {
		t.Fatal("expected 200 response to be cached")
	}
	if entry.status != http.StatusOK {
		t.Fatalf("cached status = %d, want %d", entry.status, http.StatusOK)
	}
}
