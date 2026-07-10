package service

import (
	"context"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db/caches"
	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
)

// ModelCacheService implements casc.ModelCache.
type ModelCacheService struct{}

func (ModelCacheService) EnsureInitialized(ctx context.Context) error {
	return caches.EnsureModelCachesInitialized(ctx)
}

func (ModelCacheService) GetAllSkinsForModel(fileDataID int) ([]casc.ModelSkin, error) {
	skins := m2export.GetAllSkinsForModel(uint32(fileDataID))
	return skins, nil
}
