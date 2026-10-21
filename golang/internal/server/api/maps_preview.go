package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter"
)

type mapPreviewBody struct {
	Tiles     []mapPreviewTile `json:"tiles"`
	UnitScale float64          `json:"unitScale"`
}

type mapPreviewTile struct {
	X *int `json:"x"`
	Y *int `json:"y"`
}

func registerMapPreviewRoutes(r Router, d *Deps) {
	for _, suggest := range []bool{false, true} {
		path := "/maps/{map}/preview"
		if suggest {
			path = "/maps/{map}/suggest-clamp"
		}
		r.Post(path, func(w http.ResponseWriter, req *http.Request) {
			if err := assertDesktopOnly(d.Config.IsSharedHosting); err != nil {
				sendError(w, http.StatusForbidden, err.Error())
				return
			}
			if err := ensureMapsIndex(req.Context(), d); err != nil {
				sendInternalError(w, err)
				return
			}
			id, dir, ok := resolveMap(chi.URLParam(req, "map"))
			if !ok {
				sendError(w, http.StatusNotFound, "Unknown map")
				return
			}
			var body mapPreviewBody
			if err := readJSONBody(req, &body); err != nil {
				sendError(w, http.StatusBadRequest, "Invalid request body")
				return
			}
			if err := validateMapPreviewBody(body, suggest); err != nil {
				sendError(w, http.StatusBadRequest, err.Error())
				return
			}
			mapsMu.RLock()
			entry := mapsByDir[strings.ToLower(dir)]
			npcSupported := entry != nil
			if entry != nil && entry.ExpansionID != nil {
				expansion, err := strconv.Atoi(fmt.Sprint(entry.ExpansionID))
				npcSupported = err == nil && expansion <= 2
			}
			mapsMu.RUnlock()
			if suggest && !npcSupported {
				sendError(w, http.StatusBadRequest, "NPC clamp suggestions are available through Wrath of the Lich King")
				return
			}
			info, err := d.Client.GetCASCInfo(req.Context())
			if err != nil {
				sendInternalError(w, err)
				return
			}
			if info.BuildKey == "" {
				sendError(w, http.StatusConflict, "Load a WoW build first")
				return
			}
			coords := make([]exportAdtTile, len(body.Tiles))
			for i, t := range body.Tiles {
				coords[i] = exportAdtTile{X: *t.X, Y: *t.Y}
			}
			ordered := dedupeTiles(coords)
			tiles := make([]mapexporter.TileCoord, len(ordered))
			for i, t := range ordered {
				tiles[i] = mapexporter.TileCoord{X: t.X, Y: t.Y}
			}
			ctx, cancel := context.WithCancel(req.Context())
			defer cancel()
			options := mapexporter.PreviewOptions{Client: d.Client, CacheDir: d.Config.OutputDirBrowse, BuildKey: info.BuildKey, MapID: id, MapDir: dir, Tiles: tiles, NPCsSupported: npcSupported, SuggestOnly: suggest}
			streaming := !suggest && req.URL.Query().Get("progress") == "1"
			var stream *mapPreviewStream
			if streaming {
				stream = newMapPreviewStream(w)
				options.Progress = func(progress mapexporter.PreviewProgress) {
					if stream.send(mapPreviewEvent{Progress: &progress}) != nil {
						cancel()
					}
				}
			}
			preview, err := mapexporter.BuildMapPreview(ctx, options)
			if err != nil {
				if streaming {
					if ctx.Err() == nil {
						_ = stream.send(mapPreviewEvent{Error: err.Error()})
					}
				} else if req.Context().Err() == nil {
					sendInternalError(w, err)
				}
				return
			}
			if streaming {
				_ = stream.send(mapPreviewEvent{Result: &preview})
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			if suggest {
				if len(preview.NPCs) == 0 {
					sendError(w, http.StatusBadRequest, "No NPC data in the selected cells")
					return
				}
				clamp := mapexporter.SuggestPreviewClamp(preview, body.UnitScale)
				sendJSON(w, http.StatusOK, map[string]float64{"lower": clamp.Lower, "upper": clamp.Upper})
				return
			}
			sendJSON(w, http.StatusOK, preview)
		})
	}
}

type mapPreviewEvent struct {
	Progress *mapexporter.PreviewProgress `json:"progress,omitempty"`
	Result   *mapexporter.MapPreview      `json:"result,omitempty"`
	Error    string                       `json:"error,omitempty"`
}

type mapPreviewStream struct {
	writer  http.ResponseWriter
	encoder *json.Encoder
}

func newMapPreviewStream(w http.ResponseWriter) *mapPreviewStream {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	return &mapPreviewStream{writer: w, encoder: json.NewEncoder(w)}
}

func (s *mapPreviewStream) send(event mapPreviewEvent) error {
	if err := s.encoder.Encode(event); err != nil {
		return err
	}
	return http.NewResponseController(s.writer).Flush()
}

func validateMapPreviewBody(body mapPreviewBody, suggest bool) error {
	if len(body.Tiles) == 0 || len(body.Tiles) > 4096 {
		return fmt.Errorf("select between 1 and 4096 cells")
	}
	for _, t := range body.Tiles {
		if t.X == nil || t.Y == nil {
			return fmt.Errorf("each cell requires x and y coordinates")
		}
		if *t.X < 0 || *t.X > 63 || *t.Y < 0 || *t.Y > 63 {
			return fmt.Errorf("cell coordinates must be within 0..63")
		}
	}
	if suggest && (body.UnitScale <= 0 || math.IsNaN(body.UnitScale) || math.IsInf(body.UnitScale, 0)) {
		return fmt.Errorf("unitScale must be positive")
	}
	return nil
}
