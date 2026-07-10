package httplog

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

// loggedRoutes lists every API route template. true = log (important writes only).
var loggedRoutes = map[string]bool{
	// /api
	"GET /api/get-config":                        false,
	"GET /api/browse":                            false,
	"GET /api/browse/model-skins":                false,
	"POST /api/download":                         false, // POST read: zip existing assets for download
	"GET /api/wow-config/status":                 false,
	"GET /api/wow-config/cache-size":             false,
	"POST /api/wow-config/pick-local-folder":     false, // POST read: native folder picker UI
	"POST /api/wow-config/reset":                 true,
	"POST /api/wow-config/clear-cache":           true,
	"POST /api/wow-config/discover-local":        false, // POST read: probe local installs
	"POST /api/wow-config/discover-remote":       false, // POST read: probe remote builds
	"POST /api/wow-config/apply":                 true,
	"GET /api/maps":                              false,
	"GET /api/maps/{map}/wdt-mask":               false,
	"POST /api/maps/{map}/creatures-check":       false, // POST read: validate creature spawns
	"GET /api/maps/{map}/minimap/{x}/{y}":        false,
	"POST /api/maps/{map}/generate-wc3":          true,
	"GET /api/maps/generate-wc3/status/{jobId}":  false,
	"GET /api/maps/generate-wc3/active":          false,
	"GET /api/export/character/recent":           false,
	"POST /api/export/character":                 true,
	"GET /api/export/character/status/{jobId}":   false,
	"POST /api/export/character/clean":           true,
	"GET /api/export/character/demos":            false,
	"GET /api/export/character/check-local-file": false,
	"GET /api/assets/*":                          false,
	"GET /api/browse-assets/*":                   false,
	"GET /api/texture/png/*":                     false,
	"POST /api/texture/blp":                      true,

	// /rest (wow-data-server)
	"GET /rest/getCascInfo":             false,
	"GET /rest/getConfig":               false,
	"GET /rest/searchFiles":             false,
	"GET /rest/getFileById":             false,
	"GET /rest/getFileByName":           false,
	"GET /rest/getModelSkins":           false,
	"GET /rest/initModelCaches":         false,
	"GET /rest/cascFile":                false,
	"GET /rest/download":                false,
	"GET /rest/debugMemory":             false,
	"GET /rest/getMapList":              false,
	"GET /rest/exportProgress":          false,
	"POST /rest/loadCascLocal":          true,
	"POST /rest/loadCascRemote":         true,
	"POST /rest/loadCascBuild":          true,
	"POST /rest/unloadCasc":             true,
	"POST /rest/softRestart":            true,
	"POST /rest/setConfig":              true,
	"POST /rest/charMeta":               false, // POST read: character metadata query
	"POST /rest/exportADT":              true,
	"POST /rest/finalizeExportProgress": true,
}

func routeKey(method, path string) string {
	return method + " " + normalizeRoutePath(path)
}

func normalizeRoutePath(path string) string {
	if strings.HasPrefix(path, "/api/assets/") {
		return "/api/assets/*"
	}
	if strings.HasPrefix(path, "/api/browse-assets/") {
		return "/api/browse-assets/*"
	}
	if strings.HasPrefix(path, "/api/texture/png/") {
		return "/api/texture/png/*"
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return path
	}

	switch {
	case len(parts) == 5 && parts[0] == "api" && parts[1] == "maps" && parts[3] == "minimap":
		return "/api/maps/{map}/minimap/{x}/{y}"
	case len(parts) == 4 && parts[0] == "api" && parts[1] == "maps" && parts[3] == "wdt-mask":
		return "/api/maps/{map}/wdt-mask"
	case len(parts) == 4 && parts[0] == "api" && parts[1] == "maps" && parts[3] == "generate-wc3":
		return "/api/maps/{map}/generate-wc3"
	case len(parts) == 4 && parts[0] == "api" && parts[1] == "maps" && parts[3] == "creatures-check":
		return "/api/maps/{map}/creatures-check"
	case len(parts) == 5 && parts[0] == "api" && parts[1] == "maps" && parts[2] == "generate-wc3" && parts[3] == "status":
		return "/api/maps/generate-wc3/status/{jobId}"
	case len(parts) == 5 && parts[0] == "api" && parts[1] == "export" && parts[2] == "character" && parts[3] == "status":
		return "/api/export/character/status/{jobId}"
	}

	return path
}

func isLoggedRequest(r *http.Request) bool {
	return loggedRoutes[routeKey(r.Method, r.URL.Path)]
}

// RequestLogger logs only important write APIs; everything else is silent.
func RequestLogger() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		logged := middleware.DefaultLogger(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isLoggedRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			logged.ServeHTTP(w, r)
		})
	}
}
