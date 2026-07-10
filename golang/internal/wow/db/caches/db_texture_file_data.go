package caches

import (
	"context"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

var (
	matResIDToFileDataID = make(map[uint32][]uint32)
	textureFileDataIDs   = make(map[uint32]struct{})
	textureFileDataOnce  sync.Once
	textureFileDataErr   error
)

// InitializeTextureFileData loads TextureFileData.db2.
func InitializeTextureFileData(ctx context.Context) error {
	textureFileDataOnce.Do(func() {
		log.Write("Loading texture mapping...")
		reader := db.NewWDCReader("DBFilesClient/TextureFileData.db2", nil)
		if err := reader.Parse(ctx, nil); err != nil {
			textureFileDataErr = err
			return
		}
		for textureFileDataID, row := range reader.GetAllRows() {
			textureFileDataIDs[textureFileDataID] = struct{}{}
			if toUint32(row["UsageType"]) != 0 {
				continue
			}
			materialResourcesID := toUint32(row["MaterialResourcesID"])
			matResIDToFileDataID[materialResourcesID] = append(matResIDToFileDataID[materialResourcesID], textureFileDataID)
		}
		log.Write("Loaded texture mapping for %d materials", len(matResIDToFileDataID))
	})
	return textureFileDataErr
}

// GetTextureFDIDsByMatID returns texture file data IDs for a material resource ID.
func GetTextureFDIDsByMatID(matResID uint32) []uint32 {
	return matResIDToFileDataID[matResID]
}

// EnsureTextureInitialized ensures texture file data is loaded.
func EnsureTextureInitialized(ctx context.Context) error {
	if len(matResIDToFileDataID) == 0 {
		return InitializeTextureFileData(ctx)
	}
	return nil
}

// GetTextureFileDataCacheStats returns cache stats.
func GetTextureFileDataCacheStats() (matResIDs, fileDataIDs int) {
	return len(matResIDToFileDataID), len(textureFileDataIDs)
}

// ResetTextureFileDataCache clears the texture file data cache.
func ResetTextureFileDataCache() {
	matResIDToFileDataID = make(map[uint32][]uint32)
	textureFileDataIDs = make(map[uint32]struct{})
	textureFileDataOnce = sync.Once{}
	textureFileDataErr = nil
}
