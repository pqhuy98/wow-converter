package api

import "net/http"

func registerGetConfig(r Router, d *Deps) {
	r.Get("/get-config", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "private, no-cache")
		sendJSON(w, http.StatusOK, map[string]any{
			"exportAssetDir":  d.ExportAssetDir(),
			"isSharedHosting": d.Config.IsSharedHosting,
			"isDev":           d.Config.IsDev,
			"isClassic":       d.IsClassic(req.Context()),
			"buildKey":        d.BuildKey(req.Context()),
		})
	})
}
