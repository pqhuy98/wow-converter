package transport

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFitUnixSocketPathKeepsShortPath(t *testing.T) {
	p := filepath.Join(os.TempDir(), "wow-data-server.sock")
	if len(p) > maxUnixSocketPath {
		t.Skip("TempDir already exceeds sun_path")
	}
	if got := fitUnixSocketPath(p); got != p {
		t.Fatalf("got %q want %q", got, p)
	}
}

func TestFitUnixSocketPathShortensLongDesktopPath(t *testing.T) {
	long := `C:\Users\fuzzy\Desktop\WOW-Converter export all WoW models to Warcraft 3\wow-converter\.cache\wow-data-server.sock`
	if len(long) <= maxUnixSocketPath {
		t.Fatalf("fixture too short: %d", len(long))
	}
	got := fitUnixSocketPath(long)
	if got == long {
		t.Fatal("did not shorten")
	}
	if len(got) > maxUnixSocketPath {
		t.Fatalf("still too long (%d): %q", len(got), got)
	}
}

func TestDefaultSocketPathFitsSunPathWhenEnvIsLong(t *testing.T) {
	t.Setenv("WOW_DATA_SERVER_SOCKET", `C:\Users\fuzzy\Desktop\WOW-Converter export all WoW models to Warcraft 3\wow-converter\.cache\wow-data-server.sock`)
	got := DefaultSocketPath()
	if len(got) > maxUnixSocketPath {
		t.Fatalf("too long (%d): %q", len(got), got)
	}
}

func TestFitUnixSocketPathCanListen(t *testing.T) {
	long := filepath.Join(t.TempDir(), strings.Repeat("n", 80), ".cache", "wow-data-server.sock")
	if len(long) <= maxUnixSocketPath {
		long = long + strings.Repeat("x", maxUnixSocketPath-len(long)+1)
	}
	got := fitUnixSocketPath(long)
	if len(got) > maxUnixSocketPath {
		t.Fatalf("fitted path too long (%d): %q", len(got), got)
	}
	if err := PrepareSocketPath(got); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", got)
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()
	_ = os.Remove(got)
}
