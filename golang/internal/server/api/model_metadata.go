package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/character"
)

func registerModelMetadata(r Router, d *Deps) {
	r.Post("/model/metadata", func(w http.ResponseWriter, req *http.Request) {
		raw, err := readJSONMap(req)
		if err != nil {
			sendError(w, http.StatusBadRequest, "Invalid metadata request")
			return
		}
		var issues []string
		validateCharacterObject(raw["character"], "character", &issues)
		validateOptionalString(raw, "skinId", &issues)
		if len(issues) > 0 {
			sendJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid metadata request", "issues": issues})
			return
		}
		var body struct {
			Character character.Character `json:"character"`
			SkinID    string              `json:"skinId"`
		}
		data, err := json.Marshal(raw)
		if err != nil {
			sendError(w, http.StatusBadRequest, "Invalid metadata request")
			return
		}
		if err := json.Unmarshal(data, &body); err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), time.Minute)
		defer cancel()
		if !d.IsDataServerReady(ctx) {
			sendError(w, http.StatusServiceUnavailable, "WoW data is not ready")
			return
		}
		cfg := config.DefaultConfig()
		cfg.ExportAssetDir = d.ExportAssetDir()
		exporter := character.NewCharacterExporter(cfg, d.Client)
		metadata, err := exporter.ModelMetadata(ctx, body.Character, body.SkinID)
		if err != nil {
			sendError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		sendJSON(w, http.StatusOK, metadata)
	})
}
