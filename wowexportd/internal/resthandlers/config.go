package resthandlers

import (
	"net/http"
)

func GetConfig(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key != "" {
			if v, ok := cfg.Get(key); ok {
				sendJSON(w, http.StatusOK, map[string]any{
					"id":    "CONFIG_SINGLE",
					"key":   key,
					"value": v,
				})
				return
			}
			sendJSON(w, http.StatusOK, map[string]any{
				"id":    "CONFIG_SINGLE",
				"key":   key,
				"value": nil,
			})
			return
		}
		sendJSON(w, http.StatusOK, map[string]any{
			"id":     "CONFIG_FULL",
			"config": cfg.GetAll(),
		})
	}
}

type setConfigBody struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func SetConfig(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body setConfigBody
		if err := readJSON(r, &body); err != nil || body.Key == "" {
			sendJSON(w, http.StatusBadRequest, map[string]any{
				"id": "ERR_INVALID_PARAMETERS",
				"required": map[string]any{
					"key":   "string",
					"value": "any",
				},
			})
			return
		}
		_ = cfg.Set(body.Key, body.Value)
		sendJSON(w, http.StatusOK, map[string]any{
			"id":    "CONFIG_SET_DONE",
			"key":   body.Key,
			"value": body.Value,
		})
	}
}
