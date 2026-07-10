package wowconfig

import (
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/env"
)

// Mode is local or remote WoW data source.
type Mode string

const (
	ModeLocal  Mode = "local"
	ModeRemote Mode = "remote"
)

// Config is the user-selected WoW data source.
type Config struct {
	Mode              Mode   `json:"mode"`
	InstallDirectory  string `json:"installDirectory,omitempty"`
	RegionTag         string `json:"regionTag,omitempty"`
	Product           string `json:"product"`
}

// BuildSummary mirrors CascBuildSummary from TS.
type BuildSummary struct {
	Product      string `json:"Product"`
	Region       string `json:"Region"`
	VersionsName string `json:"VersionsName"`
}

// InfoSummary mirrors CascInfoSummary from TS.
type InfoSummary struct {
	Type      string `json:"type"`
	BuildName string `json:"buildName"`
	Build     struct {
		Product      string `json:"Product"`
		Version      string `json:"Version,omitempty"`
		VersionsName string `json:"VersionsName,omitempty"`
	} `json:"build"`
}

// Status is returned by GET /api/wow-config/status.
type Status struct {
	NeedsSetup               bool         `json:"needsSetup"`
	ConfiguredFromEnv        bool         `json:"configuredFromEnv"`
	CascLoaded               bool         `json:"cascLoaded"`
	CascLoading              bool         `json:"cascLoading"`
	CascLoadingMessage       string       `json:"cascLoadingMessage,omitempty"`
	WowDataServerReachable   bool         `json:"wowDataServerReachable"`
	Config                   *Config      `json:"config"`
	CascInfo                 *InfoSummary `json:"cascInfo"`
	Error                    *string      `json:"error"`
	Products                 []constants.ProductInfo `json:"products"`
	Regions                  []string     `json:"regions"`
}

var (
	stateMu              sync.RWMutex
	memoryConfig         *Config
	lastError            *string
	applyInFlight        bool
	memoryApplyAttempted bool
	runtimeConfigOverride bool
	prevCascLoaded       bool
)

// CDNRegions lists valid CDN region tags.
var CDNRegions = []string{"eu", "us", "kr", "tw", "cn"}

func strPtr(s string) *string { return &s }

func GetEnvConfig() *Config {
	if localDir := env.CascLocalWow(); localDir != "" {
		return &Config{Mode: ModeLocal, InstallDirectory: NormalizeInstallDirectory(localDir), Product: env.CascLocalProduct()}
	}
	if region := env.CascRemoteRegion(); region != "" {
		return &Config{Mode: ModeRemote, RegionTag: region, Product: env.CascRemoteProduct()}
	}
	return nil
}

func IsEnvConfigured() bool { return GetEnvConfig() != nil }

func GetMemoryConfig() *Config {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return memoryConfig
}

func SetMemoryConfig(cfg *Config) {
	stateMu.Lock()
	defer stateMu.Unlock()
	if cfg != nil && cfg.Mode == ModeLocal {
		c := *cfg
		c.InstallDirectory = NormalizeInstallDirectory(c.InstallDirectory)
		memoryConfig = &c
		return
	}
	memoryConfig = cfg
}

func GetEffectiveConfig() *Config {
	if cfg := GetMemoryConfig(); cfg != nil {
		return cfg
	}
	return GetEnvConfig()
}

func SetError(message *string) {
	stateMu.Lock()
	defer stateMu.Unlock()
	lastError = message
}

func GetError() *string {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return lastError
}

func SetApplyInFlight(v bool) {
	stateMu.Lock()
	defer stateMu.Unlock()
	applyInFlight = v
}

func IsApplyInFlight() bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return applyInFlight
}

func ResetSession() {
	stateMu.Lock()
	defer stateMu.Unlock()
	runtimeConfigOverride = true
	memoryApplyAttempted = false
	memoryConfig = nil
	lastError = nil
}

func RuntimeConfigOverride() bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return runtimeConfigOverride
}

func SetRuntimeConfigOverride(v bool) {
	stateMu.Lock()
	defer stateMu.Unlock()
	runtimeConfigOverride = v
}

func OnCascLoadedChanged(cascLoaded bool) {
	stateMu.Lock()
	defer stateMu.Unlock()
	if prevCascLoaded && !cascLoaded && !applyInFlight {
		memoryApplyAttempted = false
		lastError = nil
	}
	prevCascLoaded = cascLoaded
}

func ShouldRetryMemoryApply() bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return !memoryApplyAttempted
}

func MarkMemoryApplyAttempted() {
	stateMu.Lock()
	defer stateMu.Unlock()
	memoryApplyAttempted = true
}

func NeedsSetup(cascLoaded bool) bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return !cascLoaded && memoryConfig == nil && (runtimeConfigOverride || !IsEnvConfigured())
}
