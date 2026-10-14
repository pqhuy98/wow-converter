package wowsource

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedRendererIsSelfContained(t *testing.T) {
	for _, name := range []string{"index.html", "capture.js", "interactive.js", "project.js", "project.wasm", "project.data"} {
		path := "/" + name
		if name == "index.html" {
			path = "/"
		}
		response := httptest.NewRecorder()
		Handler().ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 || response.Body.Len() == 0 {
			t.Fatalf("missing renderer asset %s", name)
		}
		if name == "project.wasm" && !bytes.HasPrefix(response.Body.Bytes(), []byte{0, 'a', 's', 'm'}) {
			t.Fatal("invalid WASM binary")
		}
		if name == "capture.js" && (strings.Contains(response.Body.String(), "localhost:5000") || strings.Contains(response.Body.String(), "wow.tools.local")) {
			t.Fatal("host depends on external viewer server")
		}
	}
}
