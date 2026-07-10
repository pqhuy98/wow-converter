package bootstrap

import (
	"context"
	"fmt"
	"log"

	"github.com/pqhuy98/wow-converter/internal/server/rest"
	"github.com/pqhuy98/wow-converter/internal/wow/env"
	"github.com/pqhuy98/wow-converter/internal/wow/export"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
	"github.com/pqhuy98/wow-converter/internal/wow/service"
)

// AutoLoadResult mirrors TS autoLoadCascFromEnv result.
type AutoLoadResult struct {
	Loaded    bool
	BuildName string
	Error     string
}

// AutoLoadFromEnv loads CASC when CASC_LOCAL_WOW or CASC_REMOTE_REGION is set.
func AutoLoadFromEnv(ctx context.Context, loader *service.Loader) (AutoLoadResult, error) {
	if localDir := env.CascLocalWow(); localDir != "" {
		product := env.CascLocalProduct()
		log.Printf("Auto-loading local CASC from %s (product: %s)...", localDir, product)
		if err := service.AutoLoadFromInstall(ctx, loader, localDir, product); err != nil {
			log.Printf("Auto-load of local CASC failed: %v", err)
			return AutoLoadResult{Loaded: false, Error: err.Error()}, nil
		}
		if src := server.GlobalRuntime.GetCascOptional(); src != nil {
			return AutoLoadResult{Loaded: true, BuildName: src.GetBuildName()}, nil
		}
		return AutoLoadResult{Loaded: true}, nil
	}

	if region := env.CascRemoteRegion(); region != "" {
		product := env.CascRemoteProduct()
		log.Printf("Auto-loading remote CASC (region: %s, product: %s)...", region, product)
		builds, err := loader.LoadRemote(ctx, region)
		if err != nil {
			return AutoLoadResult{Loaded: false, Error: err.Error()}, nil
		}
		idx := -1
		for i, b := range builds {
			if b.Product == product {
				idx = i
				break
			}
		}
		if idx == -1 {
			msg := fmt.Sprintf("product '%s' not found for region '%s'", product, region)
			return AutoLoadResult{Loaded: false, Error: msg}, nil
		}
		loader.SetPendingFromRemote(region, builds)
		if _, err := loader.LoadBuild(ctx, idx); err != nil {
			return AutoLoadResult{Loaded: false, Error: err.Error()}, nil
		}
		if src := server.GlobalRuntime.GetCascOptional(); src != nil {
			return AutoLoadResult{Loaded: true, BuildName: src.GetBuildName()}, nil
		}
		return AutoLoadResult{Loaded: true}, nil
	}

	return AutoLoadResult{Loaded: false}, nil
}

// NewHandler creates a REST handler with real CASC services wired.
func NewHandler() *rest.Handler {
	loader := service.NewLoader()
	h := rest.NewHandler(server.GlobalRuntime)
	h.Services = rest.Services{
		Loader:     loader,
		Listfile:   service.ListfileService{},
		ModelCache: service.ModelCacheService{},
		MapList:    service.MapListService{},
		Character:  service.CharacterMetaService{},
		Export:     service.ADTExporterService{},
		Progress:   export.ExportProgressService{},
		Memory:     service.MemoryDiagnostics{},
	}
	return h
}

// StartWowDataServer creates handler and auto-loads CASC from env.
func StartWowDataServer(ctx context.Context) (*rest.Handler, error) {
	h := NewHandler()
	if loader, ok := h.Services.Loader.(*service.Loader); ok {
		_, _ = AutoLoadFromEnv(ctx, loader)
	}
	return h, nil
}
