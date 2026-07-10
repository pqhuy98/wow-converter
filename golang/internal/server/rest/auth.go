package rest

import (
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/pqhuy98/wow-converter/internal/wow/transport"
)

const dataServerTokenHeader = "X-Wow-Data-Token"

func configuredDataServerToken() string {
	return os.Getenv("WOW_DATA_SERVER_TOKEN")
}

func dataServerAuthRequired() bool {
	if transport.UsesSocketTransport() {
		return false
	}
	return configuredDataServerToken() != ""
}

func requireDataServerToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorizeDataServerRequest(r) {
			sendJSON(w, http.StatusForbidden, map[string]any{
				"id":      "ERR_FORBIDDEN",
				"message": "missing or invalid data server token",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authorizeDataServerRequest(r *http.Request) bool {
	if !dataServerAuthRequired() {
		return true
	}
	want := configuredDataServerToken()
	got := r.Header.Get(dataServerTokenHeader)
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
