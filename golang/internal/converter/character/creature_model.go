package character

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db/caches"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

// ExportCreatureNpcAsMdl exports a creature NPC model from wowhead meta.
func ExportCreatureNpcAsMdl(ctx *ExportContext, meta CharacterData) (*mdl.MDL, error) {
	if !hasResolvedModel(meta.Model) {
		return nil, fmt.Errorf("creature NPC must contain Model")
	}
	extraGeosets := make([]int, 0)
	if meta.Creature != nil {
		for _, g := range meta.Creature.CreatureGeosetData {
			extraGeosets = append(extraGeosets, (g.GeosetIndex+1)*100+g.GeosetValue)
		}
	}
	textureIDs := make([]int, 0, len(meta.Textures))
	for _, id := range meta.Textures {
		if id > 0 {
			textureIDs = append(textureIDs, id)
		}
	}
	model, err := ExportModelFileIDAsMdl(ctx, *meta.Model, ExportModelOptions{
		TextureIDs: textureIDs, ExtraGeosets: extraGeosets,
	})
	if err != nil {
		return nil, err
	}
	if err := ApplyReplaceableTextures(ctx, model.MDL, positiveTextureMap(meta.Textures)); err != nil {
		return nil, err
	}
	return model.MDL, nil
}

// DefaultWowheadClient wraps the wowhead package.
type DefaultWowheadClient struct {
	HTTP *wowhead.HTTPClient
}

func (c DefaultWowheadClient) FetchNpcMeta(ctx context.Context, url ZamURL) (CharacterData, error) {
	_ = ctx
	exp := wowhead.Expansion(url.Expansion)
	if exp == "" {
		exp = wowhead.ExpansionLive
	}
	data, err := wowhead.FetchNpcMeta(c.HTTP, exp, url.DisplayID)
	if err != nil {
		return CharacterData{}, err
	}
	out := CharacterData{Textures: data.Textures, Model: data.Model}
	if data.Creature != nil {
		out.Creature = &CreatureMeta{}
		for _, g := range data.Creature.CreatureGeosetData {
			out.Creature.CreatureGeosetData = append(out.Creature.CreatureGeosetData, GeosetEntry{
				GeosetIndex: g.GeosetIndex, GeosetValue: g.GeosetValue,
			})
		}
	}
	return out, nil
}

// resolveNpcMetaFromDB fills model fileDataID and geoset/texture data from CASC DB2 tables.
func resolveNpcMetaFromDB(ctx context.Context, exportCtx *ExportContext, displayID int) (CharacterData, bool) {
	if exportCtx != nil && exportCtx.WowClient != nil {
		remote, err := exportCtx.WowClient.ResolveNpcDisplayMeta(ctx, displayID)
		if err == nil && remote.Found {
			return npcDisplayMetaToCharacterData(remote), true
		}
	}
	if err := caches.InitializeCreatureData(ctx); err != nil {
		return CharacterData{}, false
	}
	fileDataID, ok := caches.GetFileDataIDByDisplayID(uint32(displayID))
	if !ok {
		return CharacterData{}, false
	}
	modelID := int(fileDataID)
	meta := CharacterData{
		Model:    &modelID,
		Textures: map[string]int{},
	}
	for _, display := range caches.GetCreatureDisplaysByFileDataID(fileDataID) {
		if display.ID != uint32(displayID) {
			continue
		}
		for i, texID := range display.Textures {
			meta.Textures[strconv.Itoa(i+1)] = int(texID)
		}
		if len(display.ExtraGeosets) > 0 {
			creature := &CreatureMeta{}
			for _, geoset := range display.ExtraGeosets {
				creature.CreatureGeosetData = append(creature.CreatureGeosetData, GeosetEntry{
					GeosetIndex: int(geoset/100) - 1,
					GeosetValue: int(geoset % 100),
				})
			}
			meta.Creature = creature
		}
		break
	}
	return meta, true
}

func npcDisplayMetaToCharacterData(meta casc.NpcDisplayMeta) CharacterData {
	modelID := meta.Model
	out := CharacterData{
		Model:    &modelID,
		Textures: meta.Textures,
	}
	if len(meta.Geosets) > 0 {
		creature := &CreatureMeta{}
		for _, geoset := range meta.Geosets {
			creature.CreatureGeosetData = append(creature.CreatureGeosetData, GeosetEntry{
				GeosetIndex: geoset.GeosetIndex,
				GeosetValue: geoset.GeosetValue,
			})
		}
		out.Creature = creature
	}
	return out
}

func hasResolvedModel(model *int) bool {
	return model != nil && *model > 0
}

func positiveTextureMap(textures map[string]int) map[string]int {
	if len(textures) == 0 {
		return textures
	}
	out := make(map[string]int, len(textures))
	for k, v := range textures {
		if v > 0 {
			out[k] = v
		}
	}
	return out
}

func mergeNpcMeta(primary, fallback CharacterData, preferFallback bool) CharacterData {
	if preferFallback {
		out := primary
		if fallback.Model != nil {
			out.Model = fallback.Model
		}
		if len(fallback.Textures) > 0 {
			out.Textures = fallback.Textures
		}
		if fallback.Creature != nil {
			if out.Creature == nil {
				out.Creature = fallback.Creature
			} else if len(out.Creature.CreatureGeosetData) == 0 {
				out.Creature.CreatureGeosetData = fallback.Creature.CreatureGeosetData
			}
		}
		return out
	}
	out := primary
	if out.Model == nil {
		out.Model = fallback.Model
	}
	if len(out.Textures) == 0 && len(fallback.Textures) > 0 {
		out.Textures = fallback.Textures
	}
	if out.Creature == nil && fallback.Creature != nil {
		out.Creature = fallback.Creature
	}
	return out
}
