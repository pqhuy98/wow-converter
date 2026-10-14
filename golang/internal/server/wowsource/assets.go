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
	return http.FileServer(http.FS(sub))
}
