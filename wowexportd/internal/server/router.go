package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"wowexportd/internal/casc"
	"wowexportd/internal/core"
	"wowexportd/internal/resthandlers"
)

func newRouter(cfg *ConfigStore) http.Handler {
	mux := http.NewServeMux()
	state := core.New()
	list := casc.NewListfile()
	cache := NewResponseCache(10 * time.Second)

	// Preload listfile with TTL refresh and fallback similar to wow.export
	if err := list.LoadFromDefaultCache(); err != nil {
		_ = ensureListfileFresh(cfg)
		_ = list.LoadFromDefaultCache()
	} else if shouldRefreshListfile(cfg) {
		_ = ensureListfileFresh(cfg)
		_ = list.LoadFromDefaultCache()
	}

	// Unknown ingestion via DB caches will be implemented with real DB readers (no stubs retained)

	// GET endpoints
	mux.HandleFunc("/rest/getConfig", resthandlers.GetConfig(cfg))
	mux.HandleFunc("/rest/download", resthandlers.Download(cfg))
	// Parity: do not cache these endpoints
	mux.HandleFunc("/rest/getCascInfo", resthandlers.GetCascInfo(state, nil))
	mux.HandleFunc("/rest/searchFiles", resthandlers.SearchFiles(list, nil))
	mux.HandleFunc("/rest/getFileById", resthandlers.GetFileById(list))
	mux.HandleFunc("/rest/getFileByName", resthandlers.GetFileByName(list))
	mux.HandleFunc("/rest/getModelSkins", resthandlers.GetModelSkins())
	mux.HandleFunc("/rest/getMapList", resthandlers.GetMapList(state, list))

	// POST endpoints
	mux.HandleFunc("/rest/setConfig", resthandlers.SetConfig(cfg))
	mux.HandleFunc("/rest/loadCascLocal", resthandlers.LoadCascLocal(state))
	mux.HandleFunc("/rest/loadCascRemote", resthandlers.LoadCascRemote(state))
	mux.HandleFunc("/rest/loadCascBuild", resthandlers.LoadCascBuild(state, list, cache))
	mux.HandleFunc("/rest/exportTextures", resthandlers.ExportTextures(state, cfg, list, cache))

	// Opportunistic unknown ingestion at startup (bounded)
	go resthandlers.IngestUnknownsOnce(state, list, 5000)

	return mux
}

// shouldRefreshListfile returns true if listfile cache TTL has expired per config.
func shouldRefreshListfile(cfg *ConfigStore) bool {
	// listfileCacheRefresh in days; 0 means never refresh
	v, ok := cfg.Get("listfileCacheRefresh")
	if !ok {
		return false
	}
	days, ok := v.(int)
	if !ok || days <= 0 {
		return false
	}
	st, err := os.Stat(filepath.Join(casc.DirListfile(), "listfile.txt"))
	if err != nil {
		return true
	}
	ttl := time.Duration(days) * 24 * time.Hour
	return time.Since(st.ModTime()) > ttl
}

// ensureListfileFresh downloads listfile to cache using primary and fallback URLs.
func ensureListfileFresh(cfg *ConfigStore) error {
	primaryAny, _ := cfg.Get("listfileURL")
	fallbackAny, _ := cfg.Get("listfileFallbackURL")
	primary, _ := primaryAny.(string)
	fallback, _ := fallbackAny.(string)
	// JS replaces %s placeholder for master listfile
	if fallback != "" {
		fallback = strings.ReplaceAll(fallback, "%s", "")
	}
	urls := []string{primary}
	if fallback != "" && fallback != primary {
		urls = append(urls, fallback)
	}
	for _, u := range urls {
		if u == "" {
			continue
		}
		if b, _, err := casc.HttpGet(u); err == nil {
			_ = os.MkdirAll(casc.DirListfile(), 0o755)
			if err := os.WriteFile(filepath.Join(casc.DirListfile(), "listfile.txt"), b, 0o644); err == nil {
				return nil
			}
		}
	}
	return nil
}
