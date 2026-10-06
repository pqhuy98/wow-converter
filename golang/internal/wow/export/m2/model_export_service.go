package m2export

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/stringsort"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db/caches"
)

// GetModelDisplays returns creature/item displays for a model file data ID.
func GetModelDisplays(fileDataID uint32) []caches.ModelDisplay {
	if displays := caches.GetCreatureDisplaysByFileDataID(fileDataID); len(displays) > 0 {
		return displays
	}
	return caches.GetItemDisplaysByFileDataID(fileDataID)
}

func skinVariantKey(display caches.ModelDisplay) string {
	geosets := append([]uint32(nil), display.ExtraGeosets...)
	sort.Slice(geosets, func(i, j int) bool { return geosets[i] < geosets[j] })
	parts := make([]string, 0, len(display.Textures)+len(geosets)+1)
	for _, texture := range display.Textures {
		parts = append(parts, fmtUint(texture))
	}
	parts = append(parts, "|")
	for _, g := range geosets {
		parts = append(parts, fmtUint(g))
	}
	return strings.Join(parts, ",")
}

func getSkinForDisplay(display caches.ModelDisplay) casc.ModelSkin {
	texture := firstPositiveTexture(display.Textures)
	skinName, _ := archivecasc.GetByID(int(texture))
	if skinName != "" {
		skinName = strings.TrimSuffix(filepath.Base(skinName), ".blp")
	} else {
		skinName = "unknown_" + fmtUint(texture)
	}
	label := skinName
	// DB2 stores extra geosets as a set. Canonicalize the copy before building
	// skin IDs and labels so cache iteration order cannot change API skin IDs.
	extra := append([]uint32(nil), display.ExtraGeosets...)
	sort.Slice(extra, func(i, j int) bool { return extra[i] < extra[j] })
	if len(extra) > 0 {
		for _, g := range extra {
			skinName += fmtUint(g)
		}
		parts := make([]string, len(extra))
		for i, g := range extra {
			parts[i] = fmtUint(g)
		}
		label += " [" + strings.Join(parts, ", ") + "]"
	}
	texInts := make([]int, len(display.Textures))
	for i, t := range display.Textures {
		texInts[i] = int(t)
	}
	var extraGeosets []int
	if len(extra) > 0 {
		extraGeosets = make([]int, len(extra))
		for i, g := range extra {
			extraGeosets[i] = int(g)
		}
	}
	return casc.ModelSkin{
		ID: skinName, Label: label, DisplayID: int(display.ID), Textures: texInts, ExtraGeosets: extraGeosets,
	}
}

func firstPositiveTexture(textures []uint32) uint32 {
	for _, texture := range textures {
		if texture > 0 {
			return texture
		}
	}
	return 0
}

func preferSkin(a, b casc.ModelSkin) casc.ModelSkin {
	if len(b.Textures) != len(a.Textures) {
		if len(b.Textures) > len(a.Textures) {
			return b
		}
		return a
	}
	aKey := strings.Join(intSliceToString(a.Textures), ",")
	bKey := strings.Join(intSliceToString(b.Textures), ",")
	if aKey != bKey {
		if stringsort.Less(aKey, bKey) {
			return b
		}
		return a
	}
	if b.DisplayID > a.DisplayID {
		return b
	}
	return a
}

// GetAllSkinsForModel returns deduplicated skins for a model.
func GetAllSkinsForModel(fileDataID uint32) []casc.ModelSkin {
	displays := GetModelDisplays(fileDataID)
	byVariant := make(map[string]casc.ModelSkin)
	for _, display := range displays {
		if firstPositiveTexture(display.Textures) == 0 {
			continue
		}
		key := skinVariantKey(display)
		skin := getSkinForDisplay(display)
		if existing, ok := byVariant[key]; ok {
			byVariant[key] = preferSkin(existing, skin)
		} else {
			byVariant[key] = skin
		}
	}
	out := make([]casc.ModelSkin, 0, len(byVariant))
	for _, skin := range byVariant {
		out = append(out, skin)
	}
	disambiguateSkinVariants(out)
	stringsort.SortBy(out, func(skin casc.ModelSkin) string { return skin.Label })
	return out
}

func disambiguateSkinVariants(skins []casc.ModelSkin) {
	counts := make(map[string]int, len(skins))
	for _, skin := range skins {
		counts[skin.Label]++
	}
	for i := range skins {
		if counts[skins[i].Label] > 1 {
			skins[i].Label += fmt.Sprintf(" [display %d]", skins[i].DisplayID)
			skins[i].ID += fmt.Sprintf("_display%d", skins[i].DisplayID)
		}
	}
}

func fmtUint(v uint32) string {
	return strings.TrimSpace(strings.ReplaceAll(fmt.Sprintf("%d", v), " ", ""))
}

func intSliceToString(v []int) []string {
	out := make([]string, len(v))
	for i, n := range v {
		out[i] = fmt.Sprintf("%d", n)
	}
	return out
}
