package service

import (
	"context"
	"strconv"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db/caches"
)

// ResolveNpcDisplayMeta resolves creature model, textures, and geosets from DB2 tables.
func ResolveNpcDisplayMeta(ctx context.Context, displayID int) (casc.NpcDisplayMeta, error) {
	if err := caches.EnsureModelCachesInitialized(ctx); err != nil {
		return casc.NpcDisplayMeta{}, err
	}
	fileDataID, ok := caches.GetFileDataIDByDisplayID(uint32(displayID))
	if !ok {
		return casc.NpcDisplayMeta{Found: false}, nil
	}
	meta := casc.NpcDisplayMeta{
		Found:    true,
		Model:    int(fileDataID),
		Textures: map[string]int{},
	}
	for _, display := range caches.GetCreatureDisplaysByFileDataID(fileDataID) {
		if display.ID != uint32(displayID) {
			continue
		}
		for i, texID := range display.Textures {
			meta.Textures[strconv.Itoa(i+1)] = int(texID)
		}
		for _, geoset := range display.ExtraGeosets {
			meta.Geosets = append(meta.Geosets, casc.NpcDisplayGeoset{
				GeosetIndex: int(geoset/100) - 1,
				GeosetValue: int(geoset % 100),
			})
		}
		break
	}
	return meta, nil
}
