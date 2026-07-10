package wowconfig

import (
	"context"
	"errors"
	"fmt"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

const sharedHostingLocked = "WoW installation cannot be changed in shared hosting mode."

// Service wraps wow-data client operations for setup UI.
type Service struct {
	Client client.Client
}

func NewService(c client.Client) *Service { return &Service{Client: c} }

func (s *Service) FetchCascInfo(ctx context.Context) (*InfoSummary, error) {
	info, err := s.Client.GetCASCInfo(ctx)
	if err != nil {
		return nil, err
	}
	out := &InfoSummary{
		Type:      info.Type,
		BuildName: info.BuildName,
	}
	out.Build.Product = info.Build.Product
	out.Build.Version = info.Build.Version
	return out, nil
}

func (s *Service) DiscoverLocalBuilds(ctx context.Context, installDirectory string) ([]BuildSummary, error) {
	SetError(nil)
	builds, err := s.Client.LoadCASCLocal(ctx, installDirectory)
	if err != nil {
		return nil, discoverFailureError(true, err)
	}
	return toBuildSummaries(builds), nil
}

func (s *Service) DiscoverRemoteBuilds(ctx context.Context, regionTag string) ([]BuildSummary, error) {
	SetError(nil)
	builds, err := s.Client.LoadCASCRemote(ctx, regionTag)
	if err != nil {
		return nil, discoverFailureError(false, err)
	}
	return toBuildSummaries(builds), nil
}

func discoverFailureError(local bool, err error) error {
	msg := err.Error()
	if msg == "CASC is already active" {
		return errors.New("WoW data is already loaded. Change the installation source from setup first.")
	}
	if local {
		if msg == "invalid WoW installation directory or CDN region" {
			return errors.New("Invalid WoW installation directory")
		}
		return fmt.Errorf("Could not read WoW installation: %s", msg)
	}
	if msg == "invalid WoW installation directory or CDN region" {
		return errors.New("Invalid CDN region")
	}
	return fmt.Errorf("Could not read CDN region: %s", msg)
}

func toBuildSummaries(builds []casc.Build) []BuildSummary {
	out := make([]BuildSummary, len(builds))
	for i, b := range builds {
		out[i] = BuildSummary{
			Product:      b.Product,
			Region:       b.Region,
			VersionsName: b.VersionsName,
		}
	}
	return out
}

func findBuildIndex(builds []BuildSummary, product string) (int, error) {
	for i, b := range builds {
		if b.Product == product {
			return i, nil
		}
	}
	products := map[string]struct{}{}
	for _, b := range builds {
		products[b.Product] = struct{}{}
	}
	list := make([]string, 0, len(products))
	for p := range products {
		list = append(list, p)
	}
	return -1, fmt.Errorf("Product '%s' not found. Available: %v", product, list)
}

// Apply loads CASC for the given config.
func (s *Service) Apply(ctx context.Context, cfg Config, persist bool) (*InfoSummary, error) {
	SetApplyInFlight(true)
	SetError(nil)
	defer SetApplyInFlight(false)

	var builds []BuildSummary
	var err error
	if cfg.Mode == ModeLocal {
		builds, err = s.DiscoverLocalBuilds(ctx, cfg.InstallDirectory)
	} else {
		builds, err = s.DiscoverRemoteBuilds(ctx, cfg.RegionTag)
	}
	if err != nil {
		SetError(strPtr(err.Error()))
		return nil, err
	}

	idx, err := findBuildIndex(builds, cfg.Product)
	if err != nil {
		SetError(strPtr(err.Error()))
		return nil, err
	}

	info, err := s.Client.LoadCASCBuild(ctx, idx)
	if err != nil {
		if err.Error() == "CASC is already active" {
			if cascInfo, fetchErr := s.FetchCascInfo(ctx); fetchErr == nil {
				return cascInfo, nil
			}
		}
		msg := fmt.Errorf("Failed to load WoW data: %s", err.Error())
		SetError(strPtr(msg.Error()))
		return nil, msg
	}

	summary := &InfoSummary{Type: info.Type, BuildName: info.BuildName}
	summary.Build.Product = info.Build.Product
	summary.Build.Version = info.Build.Version

	if persist {
		SetMemoryConfig(&cfg)
		SetRuntimeConfigOverride(true)
	}
	SetError(nil)
	return summary, nil
}

// Reset unloads CASC and clears session config.
func (s *Service) Reset(ctx context.Context) error {
	result, err := s.Client.SoftRestart(ctx, false)
	if err != nil {
		if err2 := s.Client.UnloadCASC(ctx); err2 != nil {
			return fmt.Errorf("Failed to reset WoW data: %v", err2)
		}
	} else if !result.CascLoaded && result.Error != "" {
		// soft restart succeeded
	}
	ResetSession()
	return nil
}

// ClearCache unloads WoW data and deletes the repo `.cache` directory contents.
func (s *Service) ClearCache(ctx context.Context) error {
	if err := s.Reset(ctx); err != nil {
		return err
	}
	return workspace.ClearProjectCacheDir()
}

// GetStatus returns current wow-config status for the UI.
func (s *Service) GetStatus(ctx context.Context) Status {
	reachable := s.IsReachable(ctx)
	var cascInfo *InfoSummary
	cascLoaded := false
	if reachable {
		if info, err := s.FetchCascInfo(ctx); err == nil {
			cascInfo = info
			cascLoaded = true
		}
	}

	OnCascLoadedChanged(cascLoaded)

	if cascLoaded {
		SetError(nil)
	}

	errVal := GetError()
	cascLoading := IsApplyInFlight()
	cascLoadingMessage := ""
	if reachable {
		if progress, err := s.Client.GetCascLoadProgress(ctx); err == nil {
			if progress.Loading {
				cascLoading = true
			}
			cascLoadingMessage = progress.Message
		}
	}
	status := Status{
		NeedsSetup:             NeedsSetup(cascLoaded),
		ConfiguredFromEnv:      IsEnvConfigured(),
		CascLoaded:             cascLoaded,
		CascLoading:            cascLoading,
		CascLoadingMessage:     cascLoadingMessage,
		WowDataServerReachable: reachable,
		Config:                 GetEffectiveConfig(),
		CascInfo:               cascInfo,
		Error:                  errVal,
		Products:               constants.Products,
		Regions:                CDNRegions,
	}

	if cfg := GetMemoryConfig(); cfg != nil && !cascLoaded && ShouldRetryMemoryApply() {
		MarkMemoryApplyAttempted()
		if _, err := s.Apply(ctx, *cfg, true); err == nil {
			if info, err := s.FetchCascInfo(ctx); err == nil {
				status.CascInfo = info
				status.CascLoaded = true
				status.Error = nil
			}
		} else {
			status.Error = GetError()
		}
	}

	return status
}

func (s *Service) IsReachable(ctx context.Context) bool {
	_, err := s.Client.GetConfig(ctx, "exportDirectory")
	return err == nil
}

// AssertMutable returns an error when shared hosting locks config changes.
func AssertMutable(isSharedHosting bool) error {
	if isSharedHosting {
		return errors.New(sharedHostingLocked)
	}
	return nil
}

func SharedHostingLockedMessage() string { return sharedHostingLocked }
