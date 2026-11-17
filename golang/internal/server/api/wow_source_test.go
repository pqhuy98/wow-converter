package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestWowSourceEntryPointsDoNotReachDevProxyOrRedirectToDirectory(t *testing.T) {
	root := chi.NewRouter()
	registerWowSource(chi.NewRouter(), root, &Deps{})
	root.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "request reached dev proxy", http.StatusBadGateway)
	})
	query := "?model=spells%2Fbloodqueen_fire.m2&interactive=1"
	for _, test := range []struct {
		path     string
		code     int
		location string
	}{
		{"/wow-source", http.StatusTemporaryRedirect, "/wow-source/index.html" + query},
		{"/wow-source/", http.StatusOK, ""},
		{"/wow-source/index.html", http.StatusOK, ""},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			root.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path+query, nil))
			if response.Code != test.code || response.Header().Get("Location") != test.location {
				t.Fatalf("status=%d location=%q body=%s", response.Code, response.Header().Get("Location"), response.Body.String())
			}
			if test.code == http.StatusOK && !strings.Contains(response.Body.String(), `<canvas id="canvas"`) {
				t.Fatal("renderer entry page missing")
			}
		})
	}
}
