package resthandlers

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var allowedDownloadExts = map[string]bool{
	".png":  true,
	".json": true,
	".obj":  true,
	".mtl":  true,
	".csv":  true,
}

func Download(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exportDirAny, _ := cfg.Get("exportDirectory")
		exportDir, _ := exportDirAny.(string)
		if exportDir == "" {
			sendJSON(w, http.StatusServiceUnavailable, map[string]any{"id": "ERR_EXPORT_DIR_UNAVAILABLE"})
			return
		}

		requested := r.URL.Query().Get("path")
		if requested == "" || strings.Contains(requested, "\x00") {
			sendJSON(w, http.StatusBadRequest, map[string]any{
				"id":       "ERR_INVALID_PARAMETERS",
				"required": map[string]any{"path": "string (relative)"},
			})
			return
		}

		base := filepath.Clean(exportDir)
		abs := filepath.Clean(filepath.Join(base, requested))
		if !strings.HasPrefix(abs+string(os.PathSeparator), base+string(os.PathSeparator)) && abs != base {
			sendJSON(w, http.StatusForbidden, map[string]any{"id": "ERR_FORBIDDEN"})
			return
		}

		st, err := os.Stat(abs)
		if err != nil || st.IsDir() {
			sendJSON(w, http.StatusNotFound, map[string]any{"id": "ERR_NOT_FOUND"})
			return
		}

		ext := strings.ToLower(filepath.Ext(abs))
		if !allowedDownloadExts[ext] {
			sendJSON(w, http.StatusBadRequest, map[string]any{
				"id":          "ERR_INVALID_FILE_TYPE",
				"ext":         ext,
				"allowedExts": []string{".png", ".json", ".obj", ".mtl", ".csv"},
			})
			return
		}

		ct := "application/octet-stream"
		switch ext {
		case ".png":
			ct = "image/png"
		case ".json":
			ct = "application/json; charset=utf-8"
		case ".obj", ".mtl", ".csv":
			ct = "text/plain; charset=utf-8"
		default:
			if mt := mime.TypeByExtension(ext); mt != "" {
				ct = mt
			}
		}
		w.Header().Set("Content-Type", ct)
		// Serve file; on failure, emit JSON error similar to wow.export stream error handling
		f, err := os.Open(abs)
		if err != nil {
			sendJSON(w, http.StatusInternalServerError, map[string]any{"id": "ERR_INTERNAL", "message": "Failed to read file"})
			return
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			sendJSON(w, http.StatusInternalServerError, map[string]any{"id": "ERR_INTERNAL", "message": "Failed to read file"})
			return
		}
	}
}
