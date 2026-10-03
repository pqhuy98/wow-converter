package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestDownloadMissingFileIsJSON(t *testing.T) {
	dir := t.TempDir()
	apiRouter := chi.NewRouter()
	registerDownload(apiRouter, &Deps{Config: Config{OutputDir: dir, OutputDirBrowse: dir}})

	root := chi.NewRouter()
	root.Mount("/api", apiRouter)

	req := httptest.NewRequest(http.MethodPost, "/api/download", strings.NewReader(`{"files":["missing.mdx"],"source":"browse"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	const want = "{\"error\":\"File not found\"}\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
