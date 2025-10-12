package server

import (
	"encoding/json"
	"net/http"
)

// JSON helpers for server-level utilities and future middleware.
func sendJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(v)
}


