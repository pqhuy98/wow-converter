package resthandlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"wowexportd/internal/casc"
	"wowexportd/internal/core"
)

type cascState interface {
	GetActive() casc.Source
	GetPending() casc.Source
	SetPending(casc.Source)
	ActivatePending()
}

func GetCascInfo(state cascState, cache Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("getCascInfo: start")
		s := state.GetActive()
		if s == nil || !s.IsLoaded() {
			log.Printf("getCascInfo: unavailable")
			sendJSON(w, http.StatusServiceUnavailable, map[string]any{"id": "CASC_UNAVAILABLE"})
			return
		}
		// Mirror wow.export JSON shape; type must reflect source constructor name
		srcType := ""
		switch s.(type) {
		case *casc.Local:
			srcType = "CASCLocal"
		case *casc.Remote:
			srcType = "CASCRemote"
		default:
			srcType = "CASC"
		}
		resp := map[string]any{
			"id":          "CASC_INFO",
			"type":        srcType,
			"build":       s.GetSelectedBuild(),
			"buildConfig": s.GetBuildConfigMap(),
			"buildName":   s.GetBuildName(),
			"buildKey":    s.GetBuildKey(),
		}
		sendJSON(w, http.StatusOK, resp)
		log.Printf("getCascInfo: ok build=%s", s.GetBuildName())
	}
}

func LoadCascLocal(state *core.CoreView) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("loadCascLocal: start")
		if state.GetActive() != nil {
			sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_CASC_ACTIVE"})
			return
		}
		var body struct {
			InstallDirectory string `json:"installDirectory"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InstallDirectory == "" {
			sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_PARAMETERS", "required": map[string]any{"installDirectory": "string"}})
			return
		}
		src := casc.NewLocal(body.InstallDirectory)
		if err := src.Init(); err != nil {
			log.Printf("loadCascLocal: invalid install: %v", err)
			sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_INSTALL"})
			return
		}
		state.SetPending(src)
		builds := src.ListBuilds()
		sendJSON(w, http.StatusOK, map[string]any{"id": "CASC_INSTALL_BUILDS", "builds": builds})
		log.Printf("loadCascLocal: ok builds=%d", len(builds))
	}
}

func LoadCascRemote(state *core.CoreView) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("loadCascRemote: start")
		if state.GetActive() != nil {
			sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_CASC_ACTIVE"})
			return
		}
		var body struct {
			RegionTag string `json:"regionTag"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RegionTag == "" {
			sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_PARAMETERS", "required": map[string]any{"regionTag": "string"}})
			return
		}
		src := casc.NewRemote(body.RegionTag)
		if err := src.Init(); err != nil {
			log.Printf("loadCascRemote: invalid: %v", err)
			sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_INSTALL"})
			return
		}
		state.SetPending(src)
		builds := src.ListBuilds()
		sendJSON(w, http.StatusOK, map[string]any{"id": "CASC_INSTALL_BUILDS", "builds": builds})
		log.Printf("loadCascRemote: ok builds=%d", len(builds))
	}
}

func LoadCascBuild(state *core.CoreView, list Listfile, cache Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("loadCascBuild: start")
		var body struct {
			BuildIndex int `json:"buildIndex"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_PARAMETERS", "required": map[string]any{"buildIndex": "number"}})
			return
		}
		p := state.GetPending()
		if p == nil {
			log.Printf("loadCascBuild: no setup")
			sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_NO_CASC_SETUP"})
			return
		}
		if err := p.LoadBuild(body.BuildIndex); err != nil {
			log.Printf("loadCascBuild: invalid index: %v", err)
			sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_CASC_BUILD"})
			return
		}
		state.ActivatePending()
		// Apply root-filtered listfile to mirror wow.export applyPreload(rootEntries)
		if list != nil && list.IsLoaded() {
			s := state.GetActive()
			if s != nil && s.IsLoaded() {
				list.ApplyRootFilter(s.GetValidRootEntries())
			}
		}
		log.Printf("loadCascBuild: activated")
		GetCascInfo(state, nil)(w, r)
	}
}

func GetFileById(list Listfile) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !list.IsLoaded() {
			sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_LISTFILE_NOT_LOADED"})
			return
		}
		fdidStr := r.URL.Query().Get("fileDataID")
		id, _ := strconv.Atoi(fdidStr)
		if fdidStr == "" {
			sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_PARAMETERS", "required": map[string]any{"fileDataID": "number"}})
			return
		}
		name := list.GetByID(uint32(id))
		sendJSON(w, http.StatusOK, map[string]any{"id": "LISTFILE_RESULT", "fileDataID": id, "fileName": name})
	}
}

func GetFileByName(list Listfile) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !list.IsLoaded() {
			sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_LISTFILE_NOT_LOADED"})
			return
		}
		name := r.URL.Query().Get("fileName")
		if name == "" {
			sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_PARAMETERS", "required": map[string]any{"fileName": "string"}})
			return
		}
		id := list.GetByFilename(name)
		sendJSON(w, http.StatusOK, map[string]any{"id": "LISTFILE_RESULT", "fileDataID": id, "fileName": name})
	}
}

// GetModelSkins mirrors wow.export MODEL_SKINS query. Placeholder implementation; wire shape only.
func GetModelSkins() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fdidStr := r.URL.Query().Get("fileDataID")
		id, _ := strconv.Atoi(fdidStr)
		if fdidStr == "" {
			sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_PARAMETERS", "required": map[string]any{"fileDataID": "number"}})
			return
		}
		// TODO: integrate modelsService.getAllSkinsForModel parity
		sendJSON(w, http.StatusOK, map[string]any{"id": "MODEL_SKINS", "fileDataID": id, "skins": []any{}})
	}
}

// GetMapList placeholder: return ERR_NO_CASC until DB2 reader is available.
func GetMapList(state cascState, list Listfile) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := state.GetActive()
		if s == nil || !s.IsLoaded() {
			sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_NO_CASC"})
			return
		}
		// TODO: implement DB2 Map.db2 reading and listfile filtering
		sendJSON(w, http.StatusOK, map[string]any{"id": "MAP_LIST", "maps": []any{}})
	}
}
