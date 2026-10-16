package transport

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// sockaddr_un.sun_path is 108 bytes including the terminating NUL on Linux and Windows.
const maxUnixSocketPath = 107

// UsesSocketTransport reports whether wow-data-server should listen on a unix socket.
func UsesSocketTransport() bool {
	return os.Getenv("WOW_DATA_TRANSPORT") == "socket" || strings.TrimSpace(os.Getenv("WOW_DATA_SERVER_SOCKET")) != ""
}

// DefaultSocketPath returns the unix socket path for bundled mode.
// Paths longer than sun_path fall back to a hashed name under TempDir so Windows
// bind() does not fail with WSAEINVAL when the install folder is a long Desktop path.
func DefaultSocketPath() string {
	preferred := filepath.Join(workspace.FindRepoRoot(), ".cache", "wow-data-server.sock")
	if v := strings.TrimSpace(os.Getenv("WOW_DATA_SERVER_SOCKET")); v != "" {
		preferred = filepath.Clean(v)
	}
	return fitUnixSocketPath(preferred)
}

func fitUnixSocketPath(preferred string) string {
	if len(preferred) <= maxUnixSocketPath {
		return preferred
	}
	sum := fnv.New64a()
	_, _ = sum.Write([]byte(preferred))
	name := fmt.Sprintf("wcs-%x.sock", sum.Sum64())
	short := filepath.Join(os.TempDir(), name)
	if len(short) <= maxUnixSocketPath {
		return short
	}
	return filepath.Join(filepath.VolumeName(os.TempDir())+string(os.PathSeparator), name)
}

// ConfigureBundled sets env for in-process wow-data-server over unix socket only.
func ConfigureBundled() {
	_ = os.Setenv("WOW_CONVERTER_BUNDLED", "1")
	_ = os.Setenv("WOW_DATA_TRANSPORT", "socket")
	fitted := DefaultSocketPath()
	_ = os.Setenv("WOW_DATA_SERVER_SOCKET", fitted)
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
