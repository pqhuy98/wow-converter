package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter"
)

func TestBrowseAssetsStaticRoute(t *testing.T) {
	dir := t.TempDir()
	name := "sample.mdx"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("mdx"), 0o644); err != nil {
		t.Fatal(err)
	}

	apiRouter := chi.NewRouter()
	registerExportStatic(apiRouter, &Deps{Config: Config{OutputDirBrowse: dir, OutputDir: dir}})

	root := chi.NewRouter()
	root.Mount("/api", apiRouter)

	req := httptest.NewRequest(http.MethodGet, "/api/browse-assets/"+name, nil)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "mdx" {
		t.Fatalf("body = %q, want %q", got, "mdx")
	}
}

func TestBrowseSessionCleanupPreservesMapPreviewCache(t *testing.T) {
	dir := t.TempDir()
	cached := filepath.Join(dir, mapexporter.MapPreviewCacheDirectory)
	if err := os.MkdirAll(cached, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cached, "tile.bin"), []byte("cached"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "session.mdx"), []byte("temporary"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := cleanBrowseSessionAssets(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cached, "tile.bin")); err != nil {
		t.Fatal("startup discarded cached map preview", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "session.mdx")); !os.IsNotExist(err) {
		t.Fatal("ordinary browse assets were retained", err)
	}
}
