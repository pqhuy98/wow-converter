package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/pqhuy98/wow-converter/internal/server/httplimit"
)

var errRequestBodyTooLarge = errors.New("request body too large")

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func sendJSON(w http.ResponseWriter, status int, obj any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(obj)
}

func sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Cache-Control", "no-store")
	sendJSON(w, status, map[string]string{"error": message})
}

func sendInternalError(w http.ResponseWriter, err error) {
	log.Printf("api internal error: %v", err)
	sendError(w, http.StatusInternalServerError, "An internal error occurred")
}

func readJSONBody(r *http.Request, dst any) error {
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, httplimit.MaxRequestBodyBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > httplimit.MaxRequestBodyBytes {
		return errRequestBodyTooLarge
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, dst)
}

func readJSONMap(r *http.Request) (map[string]any, error) {
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, httplimit.MaxRequestBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > httplimit.MaxRequestBodyBytes {
		return nil, errRequestBodyTooLarge
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return body, nil
}
