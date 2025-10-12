package resthandlers

import (
	"net/http"
	"regexp"
)

type Listfile interface {
	IsLoaded() bool
	GetFilteredEntries(search any) []map[string]any
	GetByID(id uint32) string
	GetByFilename(name string) uint32
	ApplyRootFilter(valid []uint32)
	LoadUnknownTextures(ids []uint32) int
	LoadUnknownModels(ids []uint32) int
}

func SearchFiles(list Listfile, cache Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !list.IsLoaded() {
			sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_LISTFILE_NOT_LOADED"})
			return
		}
		search := r.URL.Query().Get("search")
		reg := r.URL.Query().Get("useRegularExpression") == "1"
		if cache != nil {
			key := "searchFiles:" + search + ":" + r.URL.RawQuery
			if status, obj, ok := cache.Get(key); ok {
				sendJSON(w, status, obj)
				return
			}
		}
		var filter any
		if reg {
			filter = regexp.MustCompile("(?i)" + search)
		} else {
			filter = search
		}
		entries := list.GetFilteredEntries(filter)
		resp := map[string]any{"id": "LISTFILE_SEARCH_RESULT", "entries": entries}
		if cache != nil {
			key := "searchFiles:" + search + ":" + r.URL.RawQuery
			cache.Set(key, http.StatusOK, resp)
		}
		sendJSON(w, http.StatusOK, resp)
	}
}
