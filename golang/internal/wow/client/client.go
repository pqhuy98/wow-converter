package client

import (
	"context"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
)

// CASCInfo mirrors the CASC_INFO response payload.
type CASCInfo struct {
	Type        string         `json:"type"`
	Build       casc.BuildInfo `json:"build"`
	BuildConfig any            `json:"buildConfig"`
	BuildName   string         `json:"buildName"`
	BuildKey    string         `json:"buildKey"`
}

// ConfigResponse is a partial or full config map.
type ConfigResponse map[string]any

// CascLoadProgress is returned by GET /rest/getCascLoadProgress.
type CascLoadProgress struct {
	Loading bool   `json:"loading"`
	Message string `json:"message"`
}

// Client is the Go counterpart of src/lib/wow-data-client/wow-data-client.ts.
type Client interface {
	WaitUntilReady(ctx context.Context) error

	GetConfig(ctx context.Context, key string) (ConfigResponse, error)
	SetConfig(ctx context.Context, key string, value any) (ConfigResponse, error)

	GetMapList(ctx context.Context) ([]casc.MapEntry, error)

	LoadCASCLocal(ctx context.Context, installDirectory string) ([]casc.Build, error)
	LoadCASCRemote(ctx context.Context, regionTag string) ([]casc.Build, error)
	LoadCASCBuild(ctx context.Context, buildIndex int) (CASCInfo, error)
	GetCASCInfo(ctx context.Context) (CASCInfo, error)
	GetCascLoadProgress(ctx context.Context) (CascLoadProgress, error)
	UnloadCASC(ctx context.Context) error
	SoftRestart(ctx context.Context, reloadEnv bool) (casc.SoftRestartResult, error)

	SearchFiles(ctx context.Context, search string, useRegex bool) ([]casc.ListfileEntry, error)
	GetFileByID(ctx context.Context, fileDataID int) (casc.ListfileEntry, error)
	GetFileByName(ctx context.Context, fileName string) (casc.ListfileEntry, error)

	GetModelSkins(ctx context.Context, fileDataID int) ([]casc.ModelSkin, error)
	InitModelCaches(ctx context.Context) error
	ResolveNpcDisplayMeta(ctx context.Context, displayID int) (casc.NpcDisplayMeta, error)

	DownloadCascFile(ctx context.Context, fileDataID int) ([]byte, error)
	DownloadExportFile(ctx context.Context, relativePath string) ([]byte, error)

	GetCharMeta(ctx context.Context, params casc.CharacterMetaParams) (casc.CharacterMetaResponse, error)
	ExportADT(ctx context.Context, params casc.ADTExportParams) (casc.ADTExportResult, error)
	ExportADTForConversion(ctx context.Context, params casc.ADTExportParams) (*exportadt.ConversionOutput, error)
	GetExportProgress(ctx context.Context, progressKey string) (*casc.ExportProgressSnapshot, error)
	FinalizeExportProgress(ctx context.Context, progressKey string) (*casc.ExportProgressSnapshot, error)
}

// DirectListfileClient scans the loaded listfile in-process without copying every entry.
type DirectListfileClient interface {
	CollectBrowseFileIndex(ctx context.Context) (models, textures []casc.ListfileEntry, err error)
	CollectMapTileFileIndex(ctx context.Context) ([]casc.ListfileEntry, error)
}
