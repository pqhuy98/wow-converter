package resthandlers

import (
	"encoding/json"
	"net/http"
)

// Config defines the small subset used by REST handlers.
type Config interface {
	Get(key string) (any, bool)
	GetAll() map[string]any
	Set(key string, value any) error
}

// Cache defines minimal response cache used by handlers.
type Cache interface {
	Get(key string) (int, any, bool)
	Set(key string, status int, obj any)
}

func sendJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
