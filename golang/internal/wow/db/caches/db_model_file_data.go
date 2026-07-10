package caches

import (
	"context"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

var (
	modelResIDToFileDataID = make(map[uint32][]uint32)
	modelFileDataIDs       = make(map[uint32]struct{})
	modelFileDataOnce      sync.Once
	modelFileDataErr       error
)

// InitializeModelFileData loads ModelFileData.db2.
func InitializeModelFileData(ctx context.Context) error {
	modelFileDataOnce.Do(func() {
		if len(modelResIDToFileDataID) > 0 {
			return
		}
		log.Write("Loading model mapping...")
		reader := db.NewWDCReader("DBFilesClient/ModelFileData.db2", nil)
		if err := reader.Parse(ctx, nil); err != nil {
			modelFileDataErr = err
			return
		}
		for modelFileDataID, row := range reader.GetAllRows() {
			modelFileDataIDs[modelFileDataID] = struct{}{}
			modelResourcesID := toUint32(row["ModelResourcesID"])
			modelResIDToFileDataID[modelResourcesID] = append(modelResIDToFileDataID[modelResourcesID], modelFileDataID)
		}
		log.Write("Loaded model mapping for %d models", len(modelResIDToFileDataID))
	})
	return modelFileDataErr
}

// GetModelFileDataID returns file data IDs for a model resource ID.
func GetModelFileDataID(modelResID uint32) []uint32 {
	return modelResIDToFileDataID[modelResID]
}

// GetModelFileDataCacheStats returns cache stats.
func GetModelFileDataCacheStats() (modelResIDs, fileDataIDs int) {
	return len(modelResIDToFileDataID), len(modelFileDataIDs)
}

// ResetModelFileDataCache clears the model file data cache.
func ResetModelFileDataCache() {
	modelResIDToFileDataID = make(map[uint32][]uint32)
	modelFileDataIDs = make(map[uint32]struct{})
	modelFileDataOnce = sync.Once{}
	modelFileDataErr = nil
}

func toUint32(v any) uint32 {
	switch x := v.(type) {
	case uint32:
		return x
	case uint8:
		return uint32(x)
	case uint16:
		return uint32(x)
	case uint64:
		return uint32(x)
	case int:
		return uint32(x)
	case int8:
		return uint32(x)
	case int16:
		return uint32(x)
	case int32:
		return uint32(x)
	case int64:
		return uint32(x)
	case float32:
		return uint32(x)
	case float64:
		return uint32(x)
	case []uint32:
		if len(x) > 0 {
			return x[0]
		}
	case []int64:
		if len(x) > 0 {
			return uint32(x[0])
		}
	}
	return 0
}
