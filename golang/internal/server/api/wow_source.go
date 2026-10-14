package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	directwmo "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/wmo"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/server/wowsource"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

func registerWowSource(api Router, root Router, d *Deps) {
	root.Handle("/wow-source/*", http.StripPrefix("/wow-source/", wowsource.Handler()))
	resolve := func(req *http.Request) (int, error) {
		if id, err := strconv.Atoi(req.URL.Query().Get("id")); err == nil && id > 0 {
			return id, nil
		}
		name := req.URL.Query().Get("name")
		if strings.HasPrefix(name, "https://") || strings.HasPrefix(name, "http://") {
			exporter := character.NewCharacterExporter(config.DefaultConfig(), d.Client)
			metadata, err := exporter.ModelMetadata(req.Context(), character.Character{Base: character.Ref{Type: "wowhead", Value: name}}, "")
			return metadata.FileDataID, err
		}
		entry, err := d.Client.GetFileByName(req.Context(), name)
		return entry.FileDataID, err
	}
	api.Get("/wow-source/raw", func(w http.ResponseWriter, req *http.Request) {
		if err := assertDesktopOnly(d.Config.IsSharedHosting); err != nil {
			sendError(w, http.StatusForbidden, err.Error())
			return
		}
		id, err := resolve(req)
		if err != nil || id <= 0 {
			sendError(w, http.StatusNotFound, "WoW asset not found")
			return
		}
		data, err := d.Client.DownloadCascFile(req.Context(), id)
		if err != nil {
			sendError(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	})
	api.Get("/wow-source/info", func(w http.ResponseWriter, req *http.Request) {
		if err := assertDesktopOnly(d.Config.IsSharedHosting); err != nil {
			sendError(w, http.StatusForbidden, err.Error())
			return
		}
		id, err := resolve(req)
		if err != nil || id <= 0 {
			sendError(w, http.StatusNotFound, "WoW model not found")
			return
		}
		entry, err := d.Client.GetFileByID(req.Context(), id)
		isWMO := strings.HasSuffix(strings.ToLower(entry.FileName), ".wmo")
		if err != nil || (!isWMO && !strings.HasSuffix(strings.ToLower(entry.FileName), ".m2")) {
			sendError(w, http.StatusBadRequest, "An M2 or WMO model is required")
			return
		}
		data, err := d.Client.DownloadCascFile(req.Context(), id)
		if err != nil {
			sendInternalError(w, err)
			return
		}
		if isWMO {
			loader := wmo.NewLoader(buffer.From(data), id, entry.FileName, false)
			if err := loader.Load(); err != nil || loader.GroupCount == 0 || len(loader.BoundingBox1) != 3 || len(loader.BoundingBox2) != 3 {
				sendError(w, http.StatusBadRequest, "A WMO root model is required")
				return
			}
			bounds := map[string]any{"Min": loader.BoundingBox1, "Max": loader.BoundingBox2}
			result := map[string]any{"kind": "wmo", "fileDataID": id, "name": entry.FileName, "bounds": bounds, "orbitBounds": bounds, "metadata": directm2.ModelMetadata{FileDataID: id, Sequences: []directm2.ModelSequence{}}}
			if req.URL.Query().Get("viewer") == "1" {
				cfg := config.DefaultConfig()
				converted, err := directwmo.ConvertWmoToMdl(req.Context(), cfg, character.CascFileSource{Client: d.Client}, directwmo.ConvertOptions{FileDataID: id, FileName: entry.FileName, Raw: data})
				if err != nil {
					sendInternalError(w, err)
					return
				}
				result["viewerBounds"] = wowSourceViewerBounds(converted.MDL.Model.Bound, cfg.RawModelScaleUp)
			}
			sendJSON(w, http.StatusOK, result)
			return
		}
		loader := m2.NewLoader(buffer.From(data), func(ctx context.Context, id uint32) ([]byte, error) {
			return d.Client.DownloadCascFile(ctx, int(id))
		})
		if err = loader.Load(req.Context()); err != nil {
			sendInternalError(w, err)
			return
		}
		exporter := character.NewCharacterExporter(config.DefaultConfig(), d.Client)
		metadata, err := exporter.ModelMetadata(req.Context(), character.Character{Base: character.LocalRef(entry.FileName)}, "")
		if err != nil {
			sendInternalError(w, err)
			return
		}
		// The vendored 2022 M2Scene's getColissionAABB centers on sequence zero
		// when inline sequences exist, despite the method's name.
		orbitBounds := loader.CollisionBox
		if len(loader.Animations) > 0 {
			orbitBounds = m2.CAABox{Min: loader.Animations[0].BoxPosMin, Max: loader.Animations[0].BoxPosMax}
		}
		result := map[string]any{"kind": "m2", "fileDataID": id, "name": entry.FileName, "bounds": loader.BoundingBox, "orbitBounds": orbitBounds, "metadata": metadata}
		if req.URL.Query().Get("viewer") == "1" {
			skins, err := d.Client.GetModelSkins(req.Context(), id)
			if err != nil {
				sendInternalError(w, err)
				return
			}
			skinID := req.URL.Query().Get("skin")
			if skinID == "" && len(skins) > 0 {
				skinID = skins[0].ID
			}
			options := make([]wowSourceSkin, 0, len(skins))
			found := skinID == ""
			for _, skin := range skins {
				replacements := make([]int, 14)
				for _, textureType := range loader.TextureTypes {
					if target := directm2.VariantTextureForType(skin.Textures, int(textureType)); target > 0 {
						replacements[int(textureType)] = target
					}
				}
				options = append(options, wowSourceSkin{ID: skin.ID, Label: skin.Label, TextureReplacements: replacements})
				if skin.ID == skinID {
					found = true
					result["textureReplacements"] = replacements
				}
			}
			if !found {
				sendError(w, http.StatusBadRequest, "Skin is not available for this model")
				return
			}
			result["skinId"], result["skins"] = skinID, options
			// Reuse the conversion graph whose geometry extents drive the MDX
			// viewer. M2 header bounds can include oversized animation volumes.
			cfg := config.DefaultConfig()
			converted, err := directm2.ConvertM2ToMdl(req.Context(), cfg, character.CascFileSource{Client: d.Client}, directm2.ConvertOptions{FileDataID: id, SkinName: skinID})
			if err != nil {
				sendInternalError(w, err)
				return
			}
			result["viewerBounds"] = wowSourceViewerBounds(converted.MDL.Model.Bound, cfg.RawModelScaleUp)
		}
		sendJSON(w, http.StatusOK, result)
	})
}

func wowSourceViewerBounds(bound components.Bound, scale float64) map[string][3]float64 {
	var min, max [3]float64
	for axis := range min {
		// Match the float32 extents serialized into the MDX artifact.
		min[axis] = float64(float32(bound.MinimumExtent[axis])) / scale
		max[axis] = float64(float32(bound.MaximumExtent[axis])) / scale
	}
	return map[string][3]float64{"Min": min, "Max": max}
}

type wowSourceSkin struct {
	ID                  string `json:"id"`
	Label               string `json:"label"`
	TextureReplacements []int  `json:"textureReplacements"`
}
