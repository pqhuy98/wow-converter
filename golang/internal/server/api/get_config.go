package api

import (
	"net/http"

	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func registerGetConfig(r Router, d *Deps) {
	r.Get("/get-config", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "private, no-cache")
		product := ""
		if info, err := d.Client.GetCASCInfo(req.Context()); err == nil {
			product = info.Build.Product
		}
		sendJSON(w, http.StatusOK, map[string]any{
			"exportAssetDir":  d.ExportAssetDir(),
			"isSharedHosting": d.Config.IsSharedHosting,
			"isDev":           d.Config.IsDev,
			"isClassic":       client.IsClassicProduct(product),
			"product":         product,
			"buildKey":        d.BuildKey(req.Context()),
			"mapGenerateHalt": true,
		})
	})
}
