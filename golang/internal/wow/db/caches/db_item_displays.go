package caches

import (
	"context"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

var (
	itemDisplays   = make(map[uint32][]ModelDisplay)
	itemOnce       sync.Once
	itemErr        error
)

// InitializeItemDisplays loads ItemDisplayInfo.db2.
func InitializeItemDisplays(ctx context.Context) error {
	itemOnce.Do(func() {
		if len(itemDisplays) > 0 {
			return
		}
		if err := InitializeModelFileData(ctx); err != nil {
			itemErr = err
			return
		}
		if err := EnsureTextureInitialized(ctx); err != nil {
			itemErr = err
			return
		}

		log.Write("Loading item textures...")
		reader := db.NewWDCReader("DBFilesClient/ItemDisplayInfo.db2", nil)
		if err := reader.Parse(ctx, nil); err != nil {
			itemErr = err
			return
		}

		for itemDisplayInfoID, row := range reader.GetAllRows() {
			modelResIDs := filterPositive(toUint32Slice(row["ModelResourcesID"]))
			if len(modelResIDs) == 0 {
				continue
			}
			matResIDs := filterPositive(toUint32Slice(row["ModelMaterialResourcesID"]))
			if len(matResIDs) == 0 {
				continue
			}
			modelFileDataIDs := GetModelFileDataID(modelResIDs[0])
			textureFileDataIDs := GetTextureFDIDsByMatID(matResIDs[0])
			if modelFileDataIDs == nil || textureFileDataIDs == nil {
				continue
			}
			for _, modelFileDataID := range modelFileDataIDs {
				display := ModelDisplay{ID: itemDisplayInfoID, Textures: textureFileDataIDs}
				itemDisplays[modelFileDataID] = append(itemDisplays[modelFileDataID], display)
			}
		}
		log.Write("Loaded textures for %d items", len(itemDisplays))
	})
	return itemErr
}

// GetItemDisplaysByFileDataID returns item skins for a file data ID.
func GetItemDisplaysByFileDataID(fileDataID uint32) []ModelDisplay {
	return itemDisplays[fileDataID]
}

// GetItemDisplayCacheStats returns cache stats.
func GetItemDisplayCacheStats() int {
	return len(itemDisplays)
}

// ResetItemDisplayCache clears item display cache.
func ResetItemDisplayCache() {
	itemDisplays = make(map[uint32][]ModelDisplay)
	itemOnce = sync.Once{}
	itemErr = nil
}
