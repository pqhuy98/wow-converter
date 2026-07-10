package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	apicasc "github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	"github.com/pqhuy98/wow-converter/internal/wow/env"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

// Loader implements single-flight CASC loading for the REST server.
type Loader struct {
	mu sync.Mutex

	loadDone chan struct{}
	loadErr  error

	pendingInstall string
	pendingRegion  string
	pendingBuilds  []apicasc.Build

	active archivecasc.CASC
}

func NewLoader() *Loader {
	return &Loader{}
}

func (l *Loader) IsLoaded() bool {
	return server.GlobalRuntime.GetCascOptional() != nil &&
		server.GlobalRuntime.GetCascOptional().IsLoaded()
}

func (l *Loader) IsLoading() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loadDone != nil
}

func (l *Loader) AwaitLoad(ctx context.Context) error {
	l.mu.Lock()
	done := l.loadDone
	l.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return l.loadErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func versionToBuild(entries []archivecasc.VersionConfigEntry) []apicasc.Build {
	out := make([]apicasc.Build, len(entries))
	for i, e := range entries {
		out[i] = apicasc.Build{
			Product:      e["Product"],
			Region:       e["Region"],
			BuildConfig:  e["BuildConfig"],
			CDNConfig:    e["CDNConfig"],
			KeyRing:      e["KeyRing"],
			BuildID:      e["BuildId"],
			VersionsName: e["VersionsName"],
		}
	}
	return out
}

func (l *Loader) LoadLocal(_ context.Context, installDirectory string) ([]apicasc.Build, error) {
	log.BeginLoadingProgress()
	defer log.EndLoadingProgress()
	casc := archivecasc.NewCASCLocal(installDirectory)
	if err := casc.Init(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	l.active = casc
	l.pendingInstall = installDirectory
	l.mu.Unlock()
	return versionToBuild(casc.Builds()), nil
}

func (l *Loader) LoadRemote(_ context.Context, regionTag string) ([]apicasc.Build, error) {
	log.BeginLoadingProgress()
	defer log.EndLoadingProgress()
	casc := archivecasc.NewCASCRemote(regionTag)
	if err := casc.Init(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	l.active = casc
	l.pendingRegion = regionTag
	l.mu.Unlock()
	return versionToBuild(casc.Builds()), nil
}

func (l *Loader) finalizeLoad(casc archivecasc.CASC, buildIndex int) (apicasc.Source, error) {
	if err := archivecasc.LoadTactKeys(); err != nil {
		return nil, err
	}
	if _, err := archivecasc.Preload(); err != nil {
		return nil, err
	}
	if err := casc.Load(buildIndex); err != nil {
		return nil, err
	}
	adapter := &SourceAdapter{CASC: casc}
	server.GlobalRuntime.SetCasc(adapter)
	if err := server.RunLoadFuncs(); err != nil {
		return nil, err
	}
	return adapter, nil
}

func (l *Loader) LoadBuild(ctx context.Context, buildIndex int) (apicasc.Source, error) {
	if src := server.GlobalRuntime.GetCascOptional(); src != nil && src.IsLoaded() {
		return src, nil
	}

	l.mu.Lock()
	if l.loadDone != nil {
		done := l.loadDone
		l.mu.Unlock()
		select {
		case <-done:
			if l.loadErr != nil {
				return nil, l.loadErr
			}
			return server.GlobalRuntime.GetCascOptional(), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	casc := l.active
	if casc == nil {
		l.mu.Unlock()
		return nil, fmt.Errorf("no pending CASC installation; call loadCascLocal or loadCascRemote first")
	}

	done := make(chan struct{})
	l.loadDone = done
	l.mu.Unlock()

	var src apicasc.Source
	var err error
	func() {
		defer close(done)
		log.BeginLoadingProgress()
		defer log.EndLoadingProgress()
		src, err = l.finalizeLoad(casc, buildIndex)
		l.loadErr = err
	}()

	l.mu.Lock()
	l.loadDone = nil
	l.mu.Unlock()

	if err != nil {
		return nil, err
	}
	return src, nil
}

func (l *Loader) Unload(_ context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.loadDone != nil {
		return fmt.Errorf("WoW data is still loading")
	}
	server.GlobalRuntime.SetCasc(nil)
	archivecasc.ResetForCascUnload()
	runtimecache.ClearWowDataServerRuntimeCaches()
	l.active = nil
	l.pendingInstall = ""
	l.pendingRegion = ""
	l.pendingBuilds = nil
	return nil
}

func (l *Loader) SoftRestart(ctx context.Context, reloadEnv bool) (apicasc.SoftRestartResult, error) {
	if err := l.Unload(ctx); err != nil {
		return apicasc.SoftRestartResult{CascLoaded: false, Error: err.Error()}, nil
	}
	if reloadEnv {
		return l.autoLoadFromEnv(ctx), nil
	}
	return apicasc.SoftRestartResult{CascLoaded: false}, nil
}

func (l *Loader) autoLoadFromEnv(ctx context.Context) apicasc.SoftRestartResult {
	if localDir := env.CascLocalWow(); localDir != "" {
		product := env.CascLocalProduct()
		if err := AutoLoadFromInstall(ctx, l, localDir, product); err != nil {
			return apicasc.SoftRestartResult{CascLoaded: false, Error: err.Error()}
		}
		if src := server.GlobalRuntime.GetCascOptional(); src != nil {
			return apicasc.SoftRestartResult{CascLoaded: true, BuildName: src.GetBuildName()}
		}
		return apicasc.SoftRestartResult{CascLoaded: true}
	}

	if region := env.CascRemoteRegion(); region != "" {
		product := env.CascRemoteProduct()
		builds, err := l.LoadRemote(ctx, region)
		if err != nil {
			return apicasc.SoftRestartResult{CascLoaded: false, Error: err.Error()}
		}
		idx := -1
		for i, b := range builds {
			if b.Product == product {
				idx = i
				break
			}
		}
		if idx == -1 {
			return apicasc.SoftRestartResult{
				CascLoaded: false,
				Error:      fmt.Sprintf("product '%s' not found for region '%s'", product, region),
			}
		}
		l.SetPendingFromRemote(region, builds)
		if _, err := l.LoadBuild(ctx, idx); err != nil {
			return apicasc.SoftRestartResult{CascLoaded: false, Error: err.Error()}
		}
		if src := server.GlobalRuntime.GetCascOptional(); src != nil {
			return apicasc.SoftRestartResult{CascLoaded: true, BuildName: src.GetBuildName()}
		}
		return apicasc.SoftRestartResult{CascLoaded: true}
	}

	return apicasc.SoftRestartResult{CascLoaded: false}
}

func (l *Loader) PendingBuilds() []apicasc.Build { return l.pendingBuilds }

func (l *Loader) SetPendingFromLocal(installDirectory string, builds []apicasc.Build) {
	l.pendingInstall = installDirectory
	l.pendingBuilds = builds
}

func (l *Loader) SetPendingFromRemote(regionTag string, builds []apicasc.Build) {
	l.pendingRegion = regionTag
	l.pendingBuilds = builds
}

func (l *Loader) ClearPending() {
	l.pendingBuilds = nil
}

// AutoLoadFromInstall loads CASC from a local install (startup path).
func AutoLoadFromInstall(ctx context.Context, l *Loader, installDir, product string) error {
	if l.IsLoaded() {
		return nil
	}
	builds, err := l.LoadLocal(ctx, installDir)
	if err != nil {
		return err
	}
	idx := -1
	for i, b := range builds {
		if b.Product == product {
			idx = i
			break
		}
	}
	if idx == -1 {
		names := make([]string, len(builds))
		for i, b := range builds {
			names[i] = b.Product
		}
		return fmt.Errorf("product '%s' not found in install. Available: %s", product, strings.Join(names, ", "))
	}
	l.SetPendingFromLocal(installDir, builds)
	_, err = l.LoadBuild(ctx, idx)
	return err
}
