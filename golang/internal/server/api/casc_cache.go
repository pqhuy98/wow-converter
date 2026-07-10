package api

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

func (d *Deps) buildKey(ctx context.Context) string {
	info, err := d.Client.GetCASCInfo(ctx)
	if err != nil {
		return ""
	}
	return info.BuildKey
}

func cascListCacheMaxAge(isDev, isSharedHosting bool) int {
	if isDev {
		return 60
	}
	if isSharedHosting {
		return 3600
	}
	return 3600
}

func cascMinimapCacheMaxAge(isDev bool) int {
	if isDev {
		return 300
	}
	return 86400
}

func etagFromParts(parts ...string) string {
	sum := md5.Sum([]byte(strings.Join(parts, "|")))
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func writeNotModified(w http.ResponseWriter, etag string) {
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusNotModified)
}

func matchNotModified(req *http.Request, etag string) bool {
	return req.Header.Get("If-None-Match") == etag
}

// applyCascBuildCache sets long-lived cache when ?build= matches the active CASC buildKey.
// Clients should pass the buildKey from /api/get-config so URLs change on version switch.
func applyCascBuildCache(w http.ResponseWriter, req *http.Request, cfg Config, activeBuild, etag string, minimap bool) {
	w.Header().Set("ETag", etag)
	reqBuild := req.URL.Query().Get("build")
	if reqBuild != "" && reqBuild == activeBuild {
		maxAge := cascListCacheMaxAge(cfg.IsDev, cfg.IsSharedHosting)
		if minimap {
			maxAge = cascMinimapCacheMaxAge(cfg.IsDev)
		}
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
		return
	}
	w.Header().Set("Cache-Control", "private, no-cache")
}

// MinimapPngPath stores decoded minimap tiles under a buildKey-specific directory.
func MinimapPngPath(assetDir, buildKey, mapDir, xs, ys string) string {
	base := filepath.Join(assetDir, "world", "minimaps")
	if buildKey != "" {
		base = filepath.Join(base, "_casc", buildKey)
	}
	return filepath.Join(base, mapDir, "map"+xs+"_"+ys+".png")
}
