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
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter/preview"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func TestMapPreviewStreamFlushAndFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	stream := newMapPreviewStream(recorder)
	p := preview.PreviewProgress{Message: "Preparing terrain", Completed: 2, Total: 4}
	if err := stream.send(mapPreviewEvent{Progress: &p}); err != nil {
		t.Fatal(err)
	}
	if !recorder.Flushed || recorder.Header().Get("Content-Type") != "application/x-ndjson" {
		t.Fatal("progress must flush immediately")
	}
	if err := stream.send(mapPreviewEvent{Result: &preview.MapPreview{}}); err != nil {
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

func TestPreviewFactionRoute(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		shared     bool
		status     int
	}{
		{"empty", `{"playerFaction":"Alliance","counts":{}}`, false, http.StatusOK},
		{"real factions Horde", `{"playerFaction":"Horde","counts":{"12":4,"14":3,"35":2}}`, false, http.StatusOK},
		{"real factions Alliance", `{"playerFaction":"Alliance","counts":{"12":4,"14":3,"35":2}}`, false, http.StatusOK},
		{"bad side", `{"playerFaction":"invalid","counts":{}}`, false, http.StatusBadRequest},
		{"bad count", `{"playerFaction":"Alliance","counts":{"12":-1}}`, false, http.StatusBadRequest},
		{"bad id", `{"playerFaction":"Alliance","counts":{"0":1}}`, false, http.StatusBadRequest},
		{"invalid JSON", `{`, false, http.StatusBadRequest},
		{"shared hosting", `{"playerFaction":"Alliance","counts":{}}`, true, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := chi.NewRouter()
			registerMapPreviewRoutes(r, &Deps{Config: Config{IsSharedHosting: tc.shared}})
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/maps/preview-factions", strings.NewReader(tc.body)))
			if rec.Code != tc.status {
				t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
			}
			if strings.HasPrefix(tc.name, "real factions") {
				var summary struct {
					Allies         []struct{ Units int } `json:"allies"`
					Enemies        []struct{ Units int } `json:"enemies"`
					NeutralHostile int                   `json:"neutralHostile"`
					NeutralPassive int                   `json:"neutralPassive"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
					t.Fatal(err)
				}
				players := summary.Allies
				if tc.name == "real factions Horde" {
					players = summary.Enemies
				}
				if len(players) != 1 || players[0].Units != 4 || summary.NeutralHostile != 3 || summary.NeutralPassive != 2 {
					t.Fatalf("incorrect faction counts: %s", rec.Body.String())
				}
			}
		})
	}
}

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
