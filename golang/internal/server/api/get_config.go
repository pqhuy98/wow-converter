package api

import "net/http"

func registerGetConfig(r Router, d *Deps) {
	r.Get("/get-config", func(w http.ResponseWriter, req *http.Request) {
		sendJSON(w, http.StatusOK, map[string]any{
			"exportAssetDir": d.ExportAssetDir(),
			"isSharedHosting": d.Config.IsSharedHosting,
			"isDev":           d.Config.IsDev,
			"isClassic":       d.IsClassic(req.Context()),
		})
	})
}
