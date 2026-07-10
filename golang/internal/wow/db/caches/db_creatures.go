package caches

import (
	"context"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

// ModelDisplay is a creature/item display entry mapped to a model fileDataID.
type ModelDisplay struct {
	ID           uint32
	ModelID      uint32
	Textures     []uint32
	ExtraGeosets []uint32
}

var (
	creatureDisplays       = make(map[uint32][]ModelDisplay)
	displayIDToFileDataID  = make(map[uint32]uint32)
	creatureInitialized    bool
	creatureOnce           sync.Once
	creatureErr            error
)

// InitializeCreatureData loads creature display tables.
func InitializeCreatureData(ctx context.Context) error {
	creatureOnce.Do(func() {
		if creatureInitialized {
			return
		}
		log.Write("Loading creature textures...")

		displayInfo := db.NewWDCReader("DBFilesClient/CreatureDisplayInfo.db2", nil)
		if err := displayInfo.Parse(ctx, nil); err != nil {
			creatureErr = err
			return
		}
		modelData := db.NewWDCReader("DBFilesClient/CreatureModelData.db2", nil)
		if err := modelData.Parse(ctx, nil); err != nil {
			creatureErr = err
			return
		}
		geosetData := db.NewWDCReader("DBFilesClient/CreatureDisplayInfoGeosetData.db2", nil)
		if err := geosetData.Parse(ctx, nil); err != nil {
			creatureErr = err
			return
		}

		creatureGeosetMap := make(map[uint32][]uint32)
		for _, geosetRow := range geosetData.GetAllRows() {
			displayInfoID := toUint32(geosetRow["CreatureDisplayInfoID"])
			geoset := (toUint32(geosetRow["GeosetIndex"])+1)*100 + toUint32(geosetRow["GeosetValue"])
			creatureGeosetMap[displayInfoID] = append(creatureGeosetMap[displayInfoID], geoset)
		}

		creatureDisplayInfoMap := make(map[uint32]ModelDisplay)
		modelIDToDisplayInfoMap := make(map[uint32][]uint32)

		for displayID, displayRow := range displayInfo.GetAllRows() {
			modelID := toUint32(displayRow["ModelID"])
			textures := filterPositive(toUint32Slice(displayRow["TextureVariationFileDataID"]))
			creatureDisplayInfoMap[displayID] = ModelDisplay{
				ID: displayID, ModelID: modelID, Textures: textures,
			}
			modelIDToDisplayInfoMap[modelID] = append(modelIDToDisplayInfoMap[modelID], displayID)
		}

		for modelID, modelRow := range modelData.GetAllRows() {
			displayIDs, ok := modelIDToDisplayInfoMap[modelID]
			if !ok {
				continue
			}
			fileDataID := toUint32(modelRow["FileDataID"])
			modelIDHasExtraGeosets := toUint32(modelRow["CreatureGeosetDataID"]) > 0
			for _, displayID := range displayIDs {
				displayIDToFileDataID[displayID] = fileDataID
				display := creatureDisplayInfoMap[displayID]
				if modelIDHasExtraGeosets {
					display.ExtraGeosets = creatureGeosetMap[displayID]
				}
				creatureDisplays[fileDataID] = append(creatureDisplays[fileDataID], display)
			}
		}

		log.Write("Loaded textures for %d creatures", len(creatureDisplays))
		creatureInitialized = true
	})
	return creatureErr
}

// GetCreatureDisplaysByFileDataID returns creature skins for a file data ID.
func GetCreatureDisplaysByFileDataID(fileDataID uint32) []ModelDisplay {
	return creatureDisplays[fileDataID]
}

// GetFileDataIDByDisplayID returns the file data ID for a display ID.
func GetFileDataIDByDisplayID(displayID uint32) (uint32, bool) {
	id, ok := displayIDToFileDataID[displayID]
	return id, ok
}

// GetCreatureCacheStats returns cache stats.
func GetCreatureCacheStats() (initialized bool, creatureDisplaysCount, displayIDMap int) {
	return creatureInitialized, len(creatureDisplays), len(displayIDToFileDataID)
}

// ResetCreatureCache clears creature cache.
func ResetCreatureCache() {
	creatureDisplays = make(map[uint32][]ModelDisplay)
	displayIDToFileDataID = make(map[uint32]uint32)
	creatureInitialized = false
	creatureOnce = sync.Once{}
	creatureErr = nil
}

func toUint32Slice(v any) []uint32 {
	switch x := v.(type) {
	case []uint32:
		return x
	case []int64:
		out := make([]uint32, len(x))
		for i, n := range x {
			out[i] = uint32(n)
		}
		return out
	case []int32:
		out := make([]uint32, len(x))
		for i, n := range x {
			out[i] = uint32(n)
		}
		return out
	default:
		return nil
	}
}

func filterPositive(ids []uint32) []uint32 {
	out := make([]uint32, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			out = append(out, id)
		}
	}
	return out
}
