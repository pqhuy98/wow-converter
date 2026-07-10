package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
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
