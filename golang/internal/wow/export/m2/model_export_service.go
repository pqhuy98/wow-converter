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
	parts := make([]string, len(geosets))
	for i, g := range geosets {
		parts[i] = fmtUint(g)
	}
	tex := uint32(0)
	if len(display.Textures) > 0 {
		tex = display.Textures[0]
	}
	return fmtUint(tex) + "|" + strings.Join(parts, ",")
}

func getSkinForDisplay(display caches.ModelDisplay) casc.ModelSkin {
	texture := display.Textures[0]
	skinName, _ := archivecasc.GetByID(int(texture))
	if skinName != "" {
		skinName = strings.TrimSuffix(filepath.Base(skinName), ".blp")
	} else {
		skinName = "unknown_" + fmtUint(texture)
	}
	label := skinName
	extra := display.ExtraGeosets
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
	texInts := make([]int, 0, len(display.Textures))
	for _, t := range display.Textures {
		if t > 0 {
			texInts = append(texInts, int(t))
		}
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
		if len(display.Textures) == 0 || display.Textures[0] == 0 {
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
	sort.Slice(out, func(i, j int) bool { return stringsort.Less(out[i].Label, out[j].Label) })
	return out
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
