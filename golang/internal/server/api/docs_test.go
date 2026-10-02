package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestDocsRouteWinsOverUICatchAll(t *testing.T) {
	r := chi.NewRouter()
	registerDocs(r)
	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "ui", http.StatusTeapot)
	}))

	page := httptest.NewRecorder()
	r.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("docs status %d", page.Code)
	}
	if !strings.Contains(page.Body.String(), "swagger-ui") {
		t.Fatal("docs page is missing the viewer")
	}

	spec := httptest.NewRecorder()
	r.ServeHTTP(spec, httptest.NewRequest(http.MethodGet, "/docs/openapi.yaml", nil))
	if spec.Code != http.StatusOK {
		t.Fatalf("spec status %d", spec.Code)
	}
	body := spec.Body.String()
	if !strings.Contains(body, "openapi: 3.0.3") || !strings.Contains(body, "/api/sound:") {
		t.Fatal("openapi spec is incomplete")
	}
	if strings.Contains(body, "/api/debugMemory") || strings.Contains(body, "/rest/") {
		t.Fatal("spec includes an internal route")
	}

	ui := httptest.NewRecorder()
	r.ServeHTTP(ui, httptest.NewRequest(http.MethodGet, "/browse", nil))
	if ui.Code != http.StatusTeapot {
		t.Fatalf("catch-all status %d", ui.Code)
	}
}
