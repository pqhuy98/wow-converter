package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func TestMapPreviewStreamFlushAndFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	stream := newMapPreviewStream(recorder)
	p := mapexporter.PreviewProgress{Message: "Preparing terrain", Completed: 2, Total: 4}
	if err := stream.send(mapPreviewEvent{Progress: &p}); err != nil {
		t.Fatal(err)
	}
	if !recorder.Flushed || recorder.Header().Get("Content-Type") != "application/x-ndjson" {
		t.Fatal("progress must flush immediately")
	}
	if err := stream.send(mapPreviewEvent{Result: &mapexporter.MapPreview{}}); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(recorder.Body)
	var progress, result mapPreviewEvent
	if err := decoder.Decode(&progress); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	if progress.Progress == nil || progress.Progress.Completed != 2 || result.Result == nil {
		t.Fatal("bad preview stream events")
	}
	if err := newMapPreviewStream(previewDisconnectedWriter{}).send(mapPreviewEvent{Progress: &p}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("disconnect must propagate to cancellation", err)
	}
}

type previewDisconnectedWriter struct{}

func (previewDisconnectedWriter) Header() http.Header       { return http.Header{} }
func (previewDisconnectedWriter) WriteHeader(int)           {}
func (previewDisconnectedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestValidateMapPreview(t *testing.T) {
	x, y, badX := 32, 48, 64
	valid := mapPreviewBody{Tiles: []mapPreviewTile{{X: &x, Y: &y}}, UnitScale: 2}
	if err := validateMapPreviewBody(valid, true); err != nil {
		t.Fatal(err)
	}
	for _, body := range []mapPreviewBody{{}, {Tiles: []mapPreviewTile{{X: &badX, Y: &y}}, UnitScale: 2}, {Tiles: []mapPreviewTile{{Y: &y}}, UnitScale: 2}, {Tiles: valid.Tiles}, {Tiles: valid.Tiles, UnitScale: math.Inf(1)}} {
		if err := validateMapPreviewBody(body, true); err == nil {
			t.Fatalf("invalid preview input accepted: %+v", body)
		}
	}
	if err := validateMapPreviewBody(mapPreviewBody{Tiles: valid.Tiles}, false); err != nil {
		t.Fatal("geometry preview does not require unit scale", err)
	}
}

type previewRouteClient struct{ client.Client }

func (previewRouteClient) GetCASCInfo(context.Context) (client.CASCInfo, error) {
	return client.CASCInfo{}, nil
}

func TestPreviewRouteContract(t *testing.T) {
	mapsMu.Lock()
	oldList, oldIndex := mapsWithTiles, mapsByDir
	classic := mapWithTiles{MapEntry: casc.MapEntry{ID: 0, Dir: "azeroth", ExpansionID: 0}}
	modern := mapWithTiles{MapEntry: casc.MapEntry{ID: 1, Dir: "modern", ExpansionID: 3}}
	mapsWithTiles = []mapWithTiles{classic, modern}
	mapsByDir = map[string]*mapWithTiles{"azeroth": &classic, "modern": &modern}
	mapsMu.Unlock()
	t.Cleanup(func() { mapsMu.Lock(); mapsWithTiles, mapsByDir = oldList, oldIndex; mapsMu.Unlock() })
	for _, shared := range []bool{false, true} {
		r := chi.NewRouter()
		registerMapPreviewRoutes(r, &Deps{Client: previewRouteClient{}, Config: Config{IsSharedHosting: shared}})
		for _, tc := range []struct {
			path, body string
			status     int
		}{
			{"azeroth/preview", `{"tiles":[{"x":32,"y":48}]}`, http.StatusConflict},
			{"azeroth/suggest-clamp", `{"tiles":[{"x":32,"y":48}],"unitScale":2}`, http.StatusConflict},
			{"azeroth/suggest-clamp", `{"tiles":[{"x":32,"y":48}]}`, http.StatusBadRequest},
			{"azeroth/preview", `{"tiles":[{"y":48}]}`, http.StatusBadRequest},
			{"modern/suggest-clamp", `{"tiles":[{"x":32,"y":48}],"unitScale":2}`, http.StatusBadRequest},
			{"unknown/preview", `{"tiles":[{"x":32,"y":48}]}`, http.StatusNotFound},
		} {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/maps/"+tc.path, strings.NewReader(tc.body)))
			want := tc.status
			if shared {
				want = http.StatusForbidden
			}
			if rec.Code != want {
				t.Fatalf("shared=%t %s: got %d, want %d: %s", shared, tc.path, rec.Code, want, rec.Body.String())
			}
		}
	}
}
