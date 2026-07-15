package rest

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/server/httplog"
	wowlog "github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/transport"
)

// Server is the wow-data-server HTTP listener.
type Server struct {
	httpServer *http.Server
	handler    *Handler
	listener   net.Listener
}

// NewServer builds a chi router with all REST routes from rest-server.ts.
func NewServer(handler *Handler, port int) *Server {
	return newServer(handler, ServerListenOptions{Port: port})
}

// ServerListenOptions configures TCP or unix socket listening.
type ServerListenOptions struct {
	Port       int
	SocketPath string
}

// NewServerWithOptions builds a server listening on TCP or a unix socket.
func NewServerWithOptions(handler *Handler, opts ServerListenOptions) *Server {
	return newServer(handler, opts)
}

func newServer(handler *Handler, opts ServerListenOptions) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(httplog.RequestLogger())

	r.Get("/rest/getCascInfo", handler.GetCascInfo)
	r.Get("/rest/getCascLoadProgress", handler.GetCascLoadProgress)
	r.Get("/rest/getConfig", handler.GetConfig)
	r.Get("/rest/searchFiles", handler.SearchFiles)
	r.Get("/rest/collectBrowseFileIndex", handler.CollectBrowseFileIndex)
	r.Get("/rest/collectMapTileFileIndex", handler.CollectMapTileFileIndex)
	r.Get("/rest/getFileById", handler.GetFileByID)
	r.Get("/rest/getFileByName", handler.GetFileByName)
	r.Get("/rest/getModelSkins", handler.GetModelSkins)
	r.Get("/rest/initModelCaches", handler.InitModelCaches)
	r.Get("/rest/resolveNpcDisplay", handler.ResolveNpcDisplay)
	r.Get("/rest/cascFile", handler.CascFile)
	r.Get("/rest/download", handler.Download)
	r.Get("/rest/getMapList", handler.GetMapList)
	r.Get("/rest/exportProgress", handler.ExportProgress)

	r.Group(func(r chi.Router) {
		r.Use(requireDataServerToken)
		if config.IsDev() {
			r.Get("/rest/debugMemory", handler.DebugMemory)
		}
		r.Post("/rest/loadCascLocal", handler.LoadCascLocal)
		r.Post("/rest/loadCascRemote", handler.LoadCascRemote)
		r.Post("/rest/loadCascBuild", handler.LoadCascBuild)
		r.Post("/rest/unloadCasc", handler.UnloadCasc)
		r.Post("/rest/softRestart", handler.SoftRestart)
		r.Post("/rest/setConfig", handler.SetConfig)
		r.Post("/rest/charMeta", handler.CharMeta)
		r.Post("/rest/exportADT", handler.ExportADT)
		r.Post("/rest/exportADTForConversion", handler.ExportADTForConversion)
		r.Post("/rest/finalizeExportProgress", handler.FinalizeExportProgress)
	})

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		sendJSON(w, http.StatusNotFound, map[string]any{"id": "ERR_NOT_FOUND"})
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		sendJSON(w, http.StatusNotFound, map[string]any{"id": "ERR_NOT_FOUND"})
	})

	host := "127.0.0.1"
	if v := os.Getenv("WOW_DATA_SERVER_HOST"); v != "" {
		host = v
	}
	addr := fmt.Sprintf("%s:%d", host, opts.Port)
	srv := &Server{
		handler: handler,
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           r,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
	if opts.SocketPath != "" {
		if err := transport.PrepareSocketPath(opts.SocketPath); err != nil {
			panic(fmt.Sprintf("prepare socket path: %v", err))
		}
		ln, err := net.Listen("unix", opts.SocketPath)
		if err != nil {
			panic(fmt.Sprintf("listen unix socket: %v", err))
		}
		srv.listener = ln
		srv.httpServer.Addr = opts.SocketPath
	}
	return srv
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	if s.listener != nil {
		if !transport.UsesSocketTransport() {
			wowlog.Write("wow-data-server listening on unix socket %s", s.httpServer.Addr)
		}
		return s.httpServer.Serve(s.listener)
	}
	wowlog.Write("wow-data-server listening on http://%s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.handler.responseCache.clear()
	err := s.httpServer.Shutdown(ctx)
	if s.listener != nil {
		_ = s.listener.Close()
		if socketPath := s.httpServer.Addr; socketPath != "" {
			_ = transport.PrepareSocketPath(socketPath)
		}
	}
	return err
}
