package config

import (
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// Config mirrors src/lib/global-config.ts Config.
type Config struct {
	TextureBaking                      TextureBakingOptions
	ExportAssetDir                     string
	AssetPrefix                        string
	RawModelScaleUp                    float64
	OverrideModels                     bool
	OverrideTextures                   bool
	MDX                                bool
	InfiniteExtentBoundRadiusThreshold float64
	IsBulkExport                       bool
	MaxTextureSize                     int
}

// TextureBakingOptions is request-scoped; zero values retain the compact default.
type TextureBakingOptions struct {
	FPS      int `json:"fps,omitempty"`
	WindowMS int `json:"windowMS,omitempty"`
}

// DefaultConfig returns converter defaults (asset dir from env or repo .cache/wow-export).
func DefaultConfig() Config {
	return Config{
		AssetPrefix:                        "wow",
		MDX:                                true,
		InfiniteExtentBoundRadiusThreshold: 2000,
		RawModelScaleUp:                    56,
		OverrideModels:                     true,
		OverrideTextures:                   false,
		ExportAssetDir:                     workspace.DefaultExportDir(),
	}
}
