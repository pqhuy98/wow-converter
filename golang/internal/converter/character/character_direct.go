package character

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	wowchar "github.com/pqhuy98/wow-converter/internal/wow/character"
	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

var defaultGeosets = map[int]struct{}{
	0: {}, 101: {}, 201: {}, 301: {}, 401: {}, 501: {}, 601: {}, 702: {}, 801: {}, 901: {},
	1001: {}, 1101: {}, 1201: {}, 1301: {}, 1400: {}, 1501: {}, 1600: {}, 1700: {}, 1801: {},
	1901: {}, 2001: {}, 2101: {}, 2201: {}, 2301: {}, 2400: {}, 2500: {}, 2601: {}, 2700: {},
	2801: {}, 2900: {}, 3000: {}, 3100: {}, 3202: {}, 3301: {}, 3401: {}, 3500: {}, 3600: {},
	3700: {}, 3801: {}, 3900: {}, 4001: {}, 4101: {}, 4201: {}, 4301: {}, 4401: {}, 4501: {},
	4601: {}, 4701: {}, 4801: {}, 4901: {}, 5001: {}, 5101: {},
}

// ExportCharacterParams describes the direct character export request body.
type ExportCharacterParams struct {
	Race                int            `json:"race"`
	Gender              int            `json:"gender"`
	FileDataIDOverride  *int           `json:"fileDataIdOverride,omitempty"`
	Customizations       map[string]int `json:"customizations"`
	CustomizationOrder   []int          `json:"-"`
	ExcludeAnimationIDs []int          `json:"excludeAnimationIds,omitempty"`
	GeosetIDs           []int          `json:"geosetIds,omitempty"`
	HideGeosetIDs       []int          `json:"hideGeosetIds,omitempty"`
}

type parsedChoiceMeta struct {
	Geosets   []int
	Materials []wowchar.CharMetaChoiceMaterial
}

const bakeCacheMax = 8

var (
	bakeCache   = map[string]map[int]directm2.DirectDataTexture{}
	bakeCacheMu sync.Mutex
)

// ClearCharacterBakeCache clears the material bake LRU cache.
func ClearCharacterBakeCache() {
	bakeCacheMu.Lock()
	bakeCache = map[string]map[int]directm2.DirectDataTexture{}
	bakeCacheMu.Unlock()
}

// BakeCacheStats returns character material bake cache sizes.
func BakeCacheStats() (entries int, textureSlots int) {
	bakeCacheMu.Lock()
	defer bakeCacheMu.Unlock()
	entries = len(bakeCache)
	for _, textures := range bakeCache {
		textureSlots += len(textures)
	}
	return entries, textureSlots
}

func init() {
	runtimecache.RegisterConverterClearHook(ClearCharacterBakeCache)
	runtimecache.RegisterMemoryStatHook(func() map[string]any {
		entries, slots := BakeCacheStats()
		return map[string]any{
			"characterBakeEntries":      entries,
			"characterBakeTextureSlots": slots,
		}
	})
	runtimecache.RegisterMemoryStatHook(func() map[string]any {
		entries, pngBytes := texturesource.CacheStats()
		return map[string]any{
			"textureRegistryEntries":  entries,
			"textureRegistryPngBytes": pngBytes,
		}
	})
}

func buildCharacterGeosetMask(skin *m2.Skin, choices map[int]parsedChoiceMeta, body ExportCharacterParams) []m2export.GeosetMaskEntry {
	subMeshes := skin.SubMeshes
	geosetGroup := func(id int) int { return (id / 100) * 100 }
	mask := make([]m2export.GeosetMaskEntry, len(subMeshes))
	for i, sub := range subMeshes {
		id := int(sub.SubmeshID)
		_, checked := defaultGeosets[id]
		mask[i] = m2export.GeosetMaskEntry{ID: id, Checked: checked}
	}
	turnOn := func(subMeshID int, turnOffOthers bool) {
		group := geosetGroup(subMeshID)
		found := false
		for i := range mask {
			if mask[i].ID == subMeshID {
				mask[i].Checked = true
				found = true
			}
		}
		if !found || group == 0 || !turnOffOthers {
			return
		}
		for i := range mask {
			if geosetGroup(mask[i].ID) == group && mask[i].ID != subMeshID {
				mask[i].Checked = false
			}
		}
	}
	hide := body.HideGeosetIDs
	for _, choiceID := range customizationChoiceOrder(body) {
		if choice, ok := choices[choiceID]; ok {
			for _, geosetID := range choice.Geosets {
				if !containsInt(hide, geosetID) {
					turnOn(geosetID, true)
				}
			}
		}
	}
	for _, geosetID := range body.GeosetIDs {
		turnOn(geosetID, true)
	}
	hideSet := map[int]struct{}{}
	for _, id := range hide {
		hideSet[id] = struct{}{}
	}
	for i := range mask {
		if _, ok := hideSet[mask[i].ID]; ok {
			mask[i].Checked = false
		}
	}
	return mask
}

// ExportCharacterDirectAsModel runs the direct character M2 -> MDL pipeline.
func ExportCharacterDirectAsModel(ctx *ExportContext, body ExportCharacterParams) (*common.Model, error) {
	meta, choices, err := resolveCharMeta(ctx, body)
	if err != nil {
		return nil, err
	}
	cacheKey := bakeCacheKey(body)
	bakeCacheMu.Lock()
	dataTextures, ok := bakeCache[cacheKey]
	if ok {
		delete(bakeCache, cacheKey)
		bakeCache[cacheKey] = dataTextures
	}
	bakeCacheMu.Unlock()
	if !ok {
		dataTextures, err = bakeCharacterMaterials(ctx, choices, body)
		if err != nil {
			return nil, err
		}
		bakeCacheMu.Lock()
		bakeCache[cacheKey] = dataTextures
		for len(bakeCache) > bakeCacheMax {
			for k := range bakeCache {
				delete(bakeCache, k)
				break
			}
		}
		bakeCacheMu.Unlock()
	}

	bodyJSON, _ := json.Marshal(body)
	sum := md5.Sum(bodyJSON)
	suffix := hex.EncodeToString(sum[:])[:8]
	exportPath := resolveCharacterExportPath(ctx.Config.ExportAssetDir, meta.FileName, suffix)

	exclude := map[int]struct{}{}
	for _, id := range body.ExcludeAnimationIDs {
		exclude[id] = struct{}{}
	}

	return ctx.AssetManager.ParseDirectOptions(context.Background(), common.DirectParseOptions{
		FileDataID: meta.FileDataID,
		GeosetMaskBuilder: func(skin *m2.Skin) []m2export.GeosetMaskEntry {
			return buildCharacterGeosetMask(skin, choices, body)
		},
		DataTextures:       dataTextures,
		ExcludeAnimIDs:     exclude,
		ExportPathOverride: exportPath,
	})
}

func resolveCharMeta(ctx *ExportContext, body ExportCharacterParams) (wowchar.MetaResult, map[int]parsedChoiceMeta, error) {
	if ctx.WowClient == nil {
		return wowchar.MetaResult{}, nil, fmt.Errorf("wow client required for character export")
	}
	params := casc.CharacterMetaParams{
		Race: body.Race, Gender: body.Gender,
		FileDataIDOverride: body.FileDataIDOverride,
		Customizations:     body.Customizations,
	}
	resp, err := ctx.WowClient.GetCharMeta(context.Background(), params)
	if err != nil {
		return wowchar.MetaResult{}, nil, err
	}
	meta := wowchar.MetaResult{
		FileDataID: resp.FileDataID, FileName: resp.FileName, TextureLayoutID: resp.TextureLayoutID,
	}
	choices := map[int]parsedChoiceMeta{}
	for key, raw := range resp.Choices {
		var choiceID int
		fmt.Sscanf(key, "%d", &choiceID)
		data, _ := json.Marshal(raw)
		var parsed wowchar.ChoiceMeta
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue
		}
		choices[choiceID] = parsedChoiceMeta{Geosets: parsed.Geosets, Materials: parsed.Materials}
		meta.Choices = map[int]wowchar.ChoiceMeta{choiceID: parsed}
	}
	return meta, choices, nil
}

func bakeCacheKey(body ExportCharacterParams) string {
	b, _ := json.Marshal(map[string]any{
		"race": body.Race, "gender": body.Gender,
		"fileDataIdOverride": body.FileDataIDOverride,
		"customizations":     body.Customizations,
	})
	return string(b)
}

func bakeCharacterMaterials(ctx *ExportContext, choices map[int]parsedChoiceMeta, body ExportCharacterParams) (map[int]directm2.DirectDataTexture, error) {
	loader := wowchar.TextureLoader(func(c context.Context, fileDataID int) ([]byte, error) {
		return ctx.WowClient.DownloadCascFile(c, fileDataID)
	})
	chrMaterials := map[int]*wowchar.MaterialRenderer{}
	chrMaterialOrder := []int{}
	for _, choiceID := range customizationChoiceOrder(body) {
		choice, ok := choices[choiceID]
		if !ok {
			continue
		}
		for _, mat := range choice.Materials {
			if mat.Section == nil {
				continue
			}
			texType := mat.Material.TextureType
			renderer := chrMaterials[texType]
			if renderer == nil {
				renderer = wowchar.NewMaterialRenderer(texType, mat.Material.Width, mat.Material.Height, loader)
				chrMaterials[texType] = renderer
				chrMaterialOrder = append(chrMaterialOrder, texType)
				renderer.Init()
			}
			filename := ""
			if mat.Filename != nil {
				filename = *mat.Filename
			}
			if err := renderer.SetTextureTarget(context.Background(), mat.CustMaterial, *mat.Section, mat.Material, mat.TextureLayer, true, filename); err != nil {
				return nil, err
			}
		}
	}
	out := map[int]directm2.DirectDataTexture{}
	for order, texType := range chrMaterialOrder {
		renderer := chrMaterials[texType]
		var filename *string
		pngBytes, err := renderer.GetPNG()
		if err != nil {
			return nil, err
		}
		if fn := renderer.FirstFilename(); fn != "" {
			filename = &fn
		}
		out[texType] = directm2.DirectDataTexture{
			Filename: filename,
			Source:   texturesource.Source{Kind: texturesource.KindPNG, PNG: pngBytes},
			Order:    order,
		}
		renderer.Dispose()
	}
	return out, nil
}

func customizationChoiceOrder(body ExportCharacterParams) []int {
	if len(body.CustomizationOrder) > 0 {
		return body.CustomizationOrder
	}
	return sortedCustomizationChoices(body.Customizations)
}

func sortedCustomizationChoices(customizations map[string]int) []int {
	keys := make([]string, 0, len(customizations))
	for key := range customizations {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		ki, kj := 0, 0
		fmt.Sscanf(keys[i], "%d", &ki)
		fmt.Sscanf(keys[j], "%d", &kj)
		return ki < kj
	})
	out := make([]int, 0, len(keys))
	for _, key := range keys {
		out = append(out, customizations[key])
	}
	return out
}

func resolveCharacterExportPath(exportRoot, fileName, suffix string) string {
	exportPath := writers.ReplaceExtension(filepath.Join(exportRoot, strings.ReplaceAll(fileName, " ", "")), ".m2")
	if suffix == "" {
		return exportPath
	}
	dir := filepath.Dir(exportPath)
	base := strings.TrimSuffix(filepath.Base(exportPath), filepath.Ext(exportPath))
	ext := filepath.Ext(exportPath)
	return filepath.Join(dir, base+"_"+suffix+ext)
}

func containsInt(slice []int, v int) bool {
	for _, n := range slice {
		if n == v {
			return true
		}
	}
	return false
}
