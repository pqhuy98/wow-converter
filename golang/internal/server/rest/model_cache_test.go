package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

type modelCacheProbe struct {
	casc.NoopModelCache
	calls int
}

func (c *modelCacheProbe) EnsureInitialized(context.Context) error {
	c.calls++
	return nil
}

type modelCacheSource struct {
	casc.Source
	loaded bool
}

func (s modelCacheSource) IsLoaded() bool { return s.loaded }

func TestInitModelCachesWaitsForLoadedSource(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source casc.Source
		status int
		calls  int
	}{{"absent", nil, http.StatusConflict, 0}, {"loading", modelCacheSource{loaded: false}, http.StatusConflict, 0}, {"ready", modelCacheSource{loaded: true}, http.StatusOK, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &server.RuntimeState{}
			runtime.SetCasc(tc.source)
			h := NewHandler(runtime)
			cache := &modelCacheProbe{}
			h.Services.ModelCache = cache
			rec := httptest.NewRecorder()
			h.InitModelCaches(rec, httptest.NewRequest(http.MethodGet, "/rest/initModelCaches", nil))
			if rec.Code != tc.status || cache.calls != tc.calls {
				t.Fatalf("status %d calls %d; want %d calls %d", rec.Code, cache.calls, tc.status, tc.calls)
			}
			if rec.Code == http.StatusConflict {
				var body struct{ ID string }
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.ID != "ERR_NO_CASC" {
					t.Fatalf("unloaded response: %s, %v", rec.Body.String(), err)
				}
			}
		})
	}
}
