package api

import (
	"net/http"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/server/pickfolder"
	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/internal/wow/wowconfig"
)

func registerWowConfig(r Router, d *Deps) {
	svc := wowconfig.NewService(d.Client)

	r.Get("/wow-config/status", func(w http.ResponseWriter, req *http.Request) {
		setNoStore(w)
		sendJSON(w, http.StatusOK, svc.GetStatus(req.Context()))
	})

	r.Get("/wow-config/cache-size", func(w http.ResponseWriter, _ *http.Request) {
		if err := assertDesktopOnly(d.Config.IsSharedHosting); err != nil {
			sendError(w, http.StatusForbidden, err.Error())
			return
		}
		bytes, err := workspace.ProjectCacheDirSize()
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		sendJSON(w, http.StatusOK, map[string]any{"bytes": bytes})
	})

	r.Post("/wow-config/pick-local-folder", func(w http.ResponseWriter, req *http.Request) {
		if err := wowconfig.AssertMutable(d.Config.IsSharedHosting); err != nil {
			status := http.StatusBadRequest
			if err.Error() == wowconfig.SharedHostingLockedMessage() {
				status = http.StatusForbidden
			}
			sendError(w, status, err.Error())
			return
		}
		body, err := readJSONMap(req)
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		startDirectory := ""
		if raw, ok := body["installDirectory"].(string); ok {
			startDirectory = wowconfig.NormalizeInstallDirectory(raw)
		}
		installDirectory, err := pickfolder.PickNativeFolder(
			"Select World of Warcraft install folder",
			startDirectory,
		)
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		if installDirectory == "" {
			sendJSON(w, http.StatusOK, map[string]any{"cancelled": true})
			return
		}
		sendJSON(w, http.StatusOK, map[string]any{"installDirectory": wowconfig.NormalizeInstallDirectory(installDirectory)})
	})

	r.Post("/wow-config/reset", func(w http.ResponseWriter, req *http.Request) {
		if err := wowconfig.AssertMutable(d.Config.IsSharedHosting); err != nil {
			status := http.StatusBadRequest
			if err.Error() == wowconfig.SharedHostingLockedMessage() {
				status = http.StatusForbidden
			}
			sendError(w, status, err.Error())
			return
		}
		if err := svc.Reset(req.Context()); err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		sendJSON(w, http.StatusOK, svc.GetStatus(req.Context()))
	})

	r.Post("/wow-config/clear-cache", func(w http.ResponseWriter, req *http.Request) {
		if err := assertDesktopOnly(d.Config.IsSharedHosting); err != nil {
			sendError(w, http.StatusForbidden, err.Error())
			return
		}
		if err := svc.ClearCache(req.Context()); err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		sendJSON(w, http.StatusOK, svc.GetStatus(req.Context()))
	})

	r.Post("/wow-config/discover-local", func(w http.ResponseWriter, req *http.Request) {
		if err := wowconfig.AssertMutable(d.Config.IsSharedHosting); err != nil {
			status := http.StatusBadRequest
			if err.Error() == wowconfig.SharedHostingLockedMessage() {
				status = http.StatusForbidden
			}
			sendError(w, status, err.Error())
			return
		}
		body, err := readJSONMap(req)
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		installDirectory, _ := body["installDirectory"].(string)
		installDirectory = wowconfig.NormalizeInstallDirectory(installDirectory)
		if installDirectory == "" {
			sendError(w, http.StatusBadRequest, "installDirectory is required")
			return
		}
		builds, err := svc.DiscoverLocalBuilds(req.Context(), installDirectory)
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		sendJSON(w, http.StatusOK, map[string]any{"builds": builds})
	})

	r.Post("/wow-config/discover-remote", func(w http.ResponseWriter, req *http.Request) {
		if err := wowconfig.AssertMutable(d.Config.IsSharedHosting); err != nil {
			status := http.StatusBadRequest
			if err.Error() == wowconfig.SharedHostingLockedMessage() {
				status = http.StatusForbidden
			}
			sendError(w, status, err.Error())
			return
		}
		body, err := readJSONMap(req)
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		regionTag, _ := body["regionTag"].(string)
		regionTag = strings.TrimSpace(regionTag)
		if regionTag == "" {
			sendError(w, http.StatusBadRequest, "regionTag is required")
			return
		}
		builds, err := svc.DiscoverRemoteBuilds(req.Context(), regionTag)
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		sendJSON(w, http.StatusOK, map[string]any{"builds": builds})
	})

	r.Post("/wow-config/apply", func(w http.ResponseWriter, req *http.Request) {
		if err := wowconfig.AssertMutable(d.Config.IsSharedHosting); err != nil {
			status := http.StatusBadRequest
			if err.Error() == wowconfig.SharedHostingLockedMessage() {
				status = http.StatusForbidden
			}
			sendError(w, status, err.Error())
			return
		}
		body, err := readJSONMap(req)
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		mode, _ := body["mode"].(string)
		product, _ := body["product"].(string)
		product = strings.TrimSpace(product)
		if mode != "local" && mode != "remote" {
			sendError(w, http.StatusBadRequest, `mode must be "local" or "remote"`)
			return
		}
		if product == "" {
			sendError(w, http.StatusBadRequest, "product is required")
			return
		}

		cfg := wowconfig.Config{Mode: wowconfig.Mode(mode), Product: product}
		if mode == "local" {
			installDirectory, _ := body["installDirectory"].(string)
			installDirectory = wowconfig.NormalizeInstallDirectory(installDirectory)
			if installDirectory == "" {
				sendError(w, http.StatusBadRequest, "installDirectory is required for local mode")
				return
			}
			cfg.InstallDirectory = installDirectory
		} else {
			regionTag, _ := body["regionTag"].(string)
			regionTag = strings.TrimSpace(regionTag)
			if regionTag == "" {
				sendError(w, http.StatusBadRequest, "regionTag is required for remote mode")
				return
			}
			cfg.RegionTag = regionTag
		}

		cascInfo, err := svc.Apply(req.Context(), cfg, true)
		if err != nil {
			sendError(w, http.StatusBadRequest, err.Error())
			return
		}
		sendJSON(w, http.StatusOK, map[string]any{
			"cascInfo": cascInfo,
			"status":   svc.GetStatus(req.Context()),
		})
	})
}
