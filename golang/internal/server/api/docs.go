package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPISpec []byte

//go:embed docs.html
var docsHTML []byte

// registerDocs serves /docs and the OpenAPI document embedded from openapi.yaml.
// The spec is written so a caller can drive the server without prior knowledge of it.
func registerDocs(r chiRouter) {
	r.Get("/docs", serveDocsPage)
	r.Get("/docs/openapi.yaml", serveOpenAPI)
}

func serveDocsPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(docsHTML)
}

func serveOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(openAPISpec)
}
