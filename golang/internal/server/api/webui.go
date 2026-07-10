package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func registerWebUI(root chiRouter, d *Deps, uiDir string) bool {
	if d.Config.IsDev {
		registerDevProxy(root)
		return true
	}
	if _, err := os.Stat(uiDir); err != nil {
		return false
	}
	absUI, _ := filepath.Abs(uiDir)
	fileServer := http.FileServer(http.Dir(absUI))
	root.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, ".") {
			fileServer.ServeHTTP(w, req)
			return
		}
		htmlFile := "index.html"
		if req.URL.Path != "/" {
			candidate := strings.TrimPrefix(req.URL.Path, "/") + ".html"
			if _, err := os.Stat(filepath.Join(absUI, candidate)); err == nil {
				htmlFile = candidate
			}
		}
		http.ServeFile(w, req, filepath.Join(absUI, htmlFile))
	})
	return true
}

func registerDevProxy(root chiRouter) {
	target, _ := url.Parse("http://localhost:3000")
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		req.Header.Set("Origin", target.String())
	}
	root.Handle("/*", proxy)
}
