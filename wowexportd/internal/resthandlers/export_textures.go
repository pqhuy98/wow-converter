package resthandlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"wowexportd/internal/casc"
	"wowexportd/internal/fileio"
	"wowexportd/internal/formats"
)

type cascActive interface {
	GetActive() casc.Source
}

// ExportTextures mirrors wow.export REST shape for /rest/exportTextures.
// Request body: { fileDataID: number | number[] }
func ExportTextures(state cascActive, cfg Config, list Listfile, cache Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := state.GetActive()
		buildKey := ""
		if s != nil {
			buildKey = s.GetBuildKey()
		}
		// Response cache based on endpoint + buildKey + body
		if cache != nil {
			var bodyCopy map[string]any
			_ = json.NewDecoder(strings.NewReader("{}")).Decode(&bodyCopy)
			key := makeCacheKey("/rest/exportTextures|"+buildKey, r)
			if status, obj, ok := cache.Get(key); ok {
				sendJSON(w, status, obj)
				return
			}
		}
		if s == nil || !s.IsLoaded() {
			status := http.StatusConflict
			obj := map[string]any{"id": "ERR_NO_CASC"}
			if cache != nil {
				cache.Set(makeCacheKey("/rest/exportTextures|"+buildKey, r), status, obj)
			}
			sendJSON(w, status, obj)
			return
		}
		var body struct {
			FileDataID any `json:"fileDataID"`
		}
		if err := readJSON(r, &body); err != nil {
			status := http.StatusBadRequest
			obj := map[string]any{"id": "ERR_INVALID_PARAMETERS", "required": map[string]any{"fileDataID": []string{"number", "number[]"}}}
			if cache != nil {
				cache.Set(makeCacheKey("/rest/exportTextures|"+buildKey, r), status, obj)
			}
			sendJSON(w, status, obj)
			return
		}
		ids := parseIDs(body.FileDataID)
		if len(ids) == 0 {
			status := http.StatusBadRequest
			obj := map[string]any{"id": "ERR_INVALID_PARAMETERS", "required": map[string]any{"fileDataID": []string{"number", "number[]"}}}
			if cache != nil {
				cache.Set(makeCacheKey("/rest/exportTextures|"+buildKey, r), status, obj)
			}
			sendJSON(w, status, obj)
			return
		}
		// Snapshot export options
		exportDirVal, _ := cfg.Get("exportDirectory")
		exportDir, _ := exportDirVal.(string)
		// Defaults for parity
		overwriteVal, _ := cfg.Get("overwriteFiles")
		overwrite, _ := overwriteVal.(bool)
		if !overwrite {
			overwrite = false
		}
		exportNamedVal, _ := cfg.Get("exportNamedFiles")
		exportNamed, _ := exportNamedVal.(bool)
		if !exportNamed {
			exportNamed = true
		}
		maskVal, _ := cfg.Get("exportChannelMask")
		mask, ok := maskVal.(float64)
		var channelMask uint8 = 0b1111
		if ok {
			channelMask = uint8(int(mask))
		}

		type entry struct {
			Type       string
			FileDataID uint32
			File       string
		}
		manifest := map[string]any{
			"type":      "TEXTURES",
			"exportID":  0, // optional in parity
			"succeeded": []entry{},
			"failed":    []map[string]any{},
		}

		for _, id := range ids {
			fileName := ""
			if list != nil {
				fileName = list.GetByID(id)
			}
			exportName := fileName
			if !exportNamed || exportName == "" {
				// Use <dir>/<id>.(blp|png) similar to wow.export fallback when unnamed
				// Keep original extension when exporting raw BLP; here we export PNG
				if idx := strings.LastIndex(fileName, "/"); idx >= 0 {
					exportName = fileName[:idx+1] + strconv.FormatUint(uint64(id), 10) + ".png"
				} else {
					exportName = strconv.FormatUint(uint64(id), 10) + ".png"
				}
			} else {
				// replace extension with .png
				dot := strings.LastIndex(exportName, ".")
				if dot >= 0 {
					exportName = exportName[:dot] + ".png"
				} else {
					exportName += ".png"
				}
			}
			abs, err := fileio.JoinExportPath(exportDir, exportName)
			if err != nil {
				continue
			}
			if err := formats.ExportTexturePNG(s, id, abs, channelMask); err != nil {
				failed := manifest["failed"].([]map[string]any)
				manifest["failed"] = append(failed, map[string]any{"type": "PNG", "fileDataID": id})
				continue
			}
			_ = fileio.EnsureDir(filepath.Dir(abs))
			// record succeeded
			succ := manifest["succeeded"].([]entry)
			manifest["succeeded"] = append(succ, entry{Type: "PNG", FileDataID: id, File: abs})
		}

		// status 200 or 422 if all failed
		succeeded := len(manifest["succeeded"].([]entry))
		failed := len(manifest["failed"].([]map[string]any))
		status := http.StatusOK
		if succeeded == 0 && failed > 0 {
			status = http.StatusUnprocessableEntity
		}
		obj := manifest
		if cache != nil {
			cache.Set(makeCacheKey("/rest/exportTextures|"+buildKey, r), status, obj)
		}
		sendJSON(w, status, obj)
	}
}

func parseIDs(v any) []uint32 {
	switch t := v.(type) {
	case float64:
		if t <= 0 {
			return nil
		}
		return []uint32{uint32(t)}
	case []any:
		out := make([]uint32, 0, len(t))
		for _, e := range t {
			if f, ok := e.(float64); ok && f > 0 {
				out = append(out, uint32(f))
			}
		}
		return out
	default:
		return nil
	}
}

func makeCacheKey(prefix string, r *http.Request) string {
	// Build a stable key from URL and body; here we use RawQuery only as body is parsed
	return prefix + ":" + r.URL.RawQuery
}
