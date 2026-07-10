package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/pqhuy98/wow-converter/internal/server/httplog"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// Router is the chi route registrar used by handlers.
type Router interface {
	Get(pattern string, handler http.HandlerFunc)
	Post(pattern string, handler http.HandlerFunc)
	Handle(pattern string, handler http.Handler)
}

type chiRouter interface {
	Router
	Mount(pattern string, handler http.Handler)
}

// Server is the wow-converter HTTP API + optional static UI.
type Server struct {
	httpServer *http.Server
	withUI     bool
}

// NewServer builds the chi router with all /api routes from src/server/start.ts.
func NewServer(d *Deps) *Server {
	cfg := d.Config

	apiRouter := chi.NewRouter()
	registerGetConfig(apiRouter, d)
	registerBrowse(apiRouter, d)
	registerDownload(apiRouter, d)
	registerWowConfig(apiRouter, d)
	registerMaps(apiRouter, d)
	registerExportCharacter(apiRouter, d)
	registerExportTexture(apiRouter, d)

	root := chi.NewRouter()
	root.Use(middleware.Recoverer)
	root.Use(middleware.RequestID)
	root.Use(middleware.RealIP)
	root.Use(httplog.RequestLogger())
	root.Use(middleware.Compress(5))
	root.Use(securityHeaders)
	if !cfg.IsSharedHosting || cfg.IsDev {
		root.Use(corsMiddleware)
	}

	root.Mount("/api", apiRouter)

	uiDir := resolveUIDir()
	withUI := registerWebUI(root, d, uiDir)

	s := &Server{withUI: withUI, httpServer: &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.ListenHost, cfg.Port),
		Handler:           root,
		ReadHeaderTimeout: 10 * time.Second,
	}}
	return s
}

func resolveUIDir() string {
	return filepath.Join(workspace.AppRoot(), "webui", "out")
}

// ListenAndServe starts the converter server.
func (s *Server) ListenAndServe() error {
	if !s.withUI {
		log.Printf("Found no UI, serving only REST API at http://%s/", s.httpServer.Addr)
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Handler returns the root HTTP handler (for tests).
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && isAllowedCorsOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isAllowedCorsOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1"
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		next.ServeHTTP(w, r)
	})
}
