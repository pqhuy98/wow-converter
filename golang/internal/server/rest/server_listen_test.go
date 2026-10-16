package rest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNewServerWithOptionsUnixListenFailureDoesNotPanic(t *testing.T) {
	long := filepath.Join(t.TempDir(), strings.Repeat("n", 80), "wow-data-server.sock")
	var srv *Server
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("panicked: %v", rec)
			}
		}()
		srv = NewServerWithOptions(&Handler{}, ServerListenOptions{SocketPath: long})
	}()
	if err := srv.ListenAndServe(); err == nil {
		t.Fatal("expected unix listen error")
	}
}
