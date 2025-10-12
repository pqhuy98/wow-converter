package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Server struct {
	port int
	http *http.Server
	cfg  *ConfigStore
}

func New(port int) *Server {
	cfg := NewConfigStore()
	r := newRouter(cfg)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           withLogging(r),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return &Server{port: port, http: srv, cfg: cfg}
}

func (s *Server) Start() error {
	log.Printf("Listening for REST requests on port %d", s.port)
	return s.http.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
