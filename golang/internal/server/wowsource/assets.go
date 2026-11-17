// Package wowsource serves the original WoW renderer without an external viewer server.
package wowsource

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/*
var assets embed.FS

func Handler() http.Handler {
	sub, _ := fs.Sub(assets, "assets")
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Keep the entry URL: Next dev removes the directory redirect's slash.
		if r.URL.Path == "/index.html" {
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	})
}
