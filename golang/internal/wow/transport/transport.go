package transport

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// UsesSocketTransport reports whether wow-data-server should listen on a unix socket.
func UsesSocketTransport() bool {
	return os.Getenv("WOW_DATA_TRANSPORT") == "socket" || strings.TrimSpace(os.Getenv("WOW_DATA_SERVER_SOCKET")) != ""
}

// DefaultSocketPath returns the unix socket path for bundled mode.
func DefaultSocketPath() string {
	if v := strings.TrimSpace(os.Getenv("WOW_DATA_SERVER_SOCKET")); v != "" {
		return filepath.Clean(v)
	}
	return filepath.Join(workspace.FindRepoRoot(), ".cache", "wow-data-server.sock")
}

// ConfigureBundled sets env for in-process wow-data-server over unix socket only.
func ConfigureBundled() {
	_ = os.Setenv("WOW_CONVERTER_BUNDLED", "1")
	_ = os.Setenv("WOW_DATA_TRANSPORT", "socket")
	if strings.TrimSpace(os.Getenv("WOW_DATA_SERVER_SOCKET")) == "" {
		_ = os.Setenv("WOW_DATA_SERVER_SOCKET", DefaultSocketPath())
	}
	_ = os.Unsetenv("WOW_DATA_SERVER_URL")
	_ = os.Unsetenv("WOW_DATA_SERVER_PORT")
}

// PrepareSocketPath ensures the socket directory exists and removes a stale socket file.
func PrepareSocketPath(socketPath string) error {
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	_ = os.Remove(socketPath)
	return nil
}
