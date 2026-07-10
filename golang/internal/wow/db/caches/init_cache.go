package caches

import (
	"context"
	"sync"
)

var (
	modelInitOnce     sync.Once
	modelInitErr      error
	modelCachesLoaded bool
)

// EnsureModelCachesInitialized loads DB2 skin caches in dependency order.
func EnsureModelCachesInitialized(ctx context.Context) error {
	modelInitOnce.Do(func() {
		if err := InitializeModelFileData(ctx); err != nil {
			modelInitErr = err
			return
		}
		if err := InitializeTextureFileData(ctx); err != nil {
			modelInitErr = err
			return
		}
		if err := InitializeItemDisplays(ctx); err != nil {
			modelInitErr = err
			return
		}
		if err := InitializeCreatureData(ctx); err != nil {
			modelInitErr = err
			return
		}
		modelCachesLoaded = true
	})
	return modelInitErr
}

// ResetDBCaches clears all DB caches.
func ResetDBCaches() {
	ResetCreatureCache()
	ResetItemDisplayCache()
	ResetModelFileDataCache()
	ResetTextureFileDataCache()
	modelInitOnce = sync.Once{}
	modelInitErr = nil
	modelCachesLoaded = false
}

// ModelCachesInitialized reports whether model DB caches loaded successfully.
func ModelCachesInitialized() bool {
	return modelCachesLoaded
}
