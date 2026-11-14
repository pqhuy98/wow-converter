package formats

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type downloadTransport func(*http.Request) (*http.Response, error)

func (f downloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type failedDownloadReader struct{}

func (failedDownloadReader) Read([]byte) (int, error) { return 0, errors.New("broken download") }
func (failedDownloadReader) Close() error             { return nil }

func TestDiskDownloadPreservesCacheAndUsesFallback(t *testing.T) {
	previous := httpClient
	t.Cleanup(func() { httpClient = previous })
	path := filepath.Join(t.TempDir(), "listfile.txt")
	if err := os.WriteFile(path, []byte("old cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	httpClient = &http.Client{Transport: downloadTransport(func(r *http.Request) (*http.Response, error) {
		var body io.ReadCloser = failedDownloadReader{}
		if r.URL.Path == "/good" {
			body = io.NopCloser(strings.NewReader("new cache"))
		}
		return &http.Response{StatusCode: 200, Body: body, Header: make(http.Header)}, nil
	})}
	if err := DownloadFileToDisk([]string{"https://github.com/broken"}, path); err == nil {
		t.Fatal("accepted incomplete response")
	}
	if data, _ := os.ReadFile(path); string(data) != "old cache" {
		t.Fatal("failed download replaced cache")
	}
	if err := DownloadFileToDisk([]string{"https://github.com/broken", "https://github.com/good"}, path); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "new cache" {
		t.Fatal("fallback did not replace cache")
	}
	files, _ := os.ReadDir(filepath.Dir(path))
	if len(files) != 1 {
		t.Fatal("temporary download leaked")
	}
}
