package casc

import "context"

// Build describes a WoW installation build entry.
type Build struct {
	Product      string `json:"Product"`
	Region       string `json:"Region"`
	BuildConfig  string `json:"BuildConfig"`
	CDNConfig    string `json:"CDNConfig"`
	KeyRing      string `json:"KeyRing"`
	BuildID      string `json:"BuildId"`
	VersionsName string `json:"VersionsName"`
}

// BuildInfo is the loaded build metadata returned by CASC_INFO.
type BuildInfo struct {
	Product       string `json:"Product"`
	Version       string `json:"Version"`
	VersionsName  string `json:"VersionsName,omitempty"`
	Active        string `json:"Active"`
	Armadillo     string `json:"Armadillo"`
	Branch        string `json:"Branch"`
	BuildKey      string `json:"BuildKey"`
	CDNHosts      string `json:"CDNHosts"`
	CDNKey        string `json:"CDNKey"`
	CDNPath       string `json:"CDNPath"`
	CDNServers    string `json:"CDNServers"`
	IMSize        string `json:"IMSize"`
	InstallKey    string `json:"InstallKey"`
	KeyRing       string `json:"KeyRing"`
	LastActivated string `json:"LastActivated"`
	Tags          string `json:"Tags"`
}

// Source is the active CASC archive (mirrors TS CASC interface).
type Source interface {
	IsLoaded() bool
	TypeName() string
	Build() BuildInfo
	BuildConfig() any
	GetBuildName() string
	GetBuildKey() string
	GetFile(ctx context.Context, fileDataID int) ([]byte, error)
	// GetFilePartial loads a CASC file with partial BLTE decrypt (DB2 tables with encrypted sections).
	GetFilePartial(ctx context.Context, fileDataID int) ([]byte, error)
}

// Loader manages CASC setup and loading lifecycle.
type Loader interface {
	IsLoaded() bool
	IsLoading() bool
	AwaitLoad(ctx context.Context) error

	LoadLocal(ctx context.Context, installDirectory string) ([]Build, error)
	LoadRemote(ctx context.Context, regionTag string) ([]Build, error)
	LoadBuild(ctx context.Context, buildIndex int) (Source, error)
	Unload(ctx context.Context) error
	SoftRestart(ctx context.Context, reloadEnv bool) (SoftRestartResult, error)

	PendingBuilds() []Build
	SetPendingFromLocal(installDirectory string, builds []Build)
	SetPendingFromRemote(regionTag string, builds []Build)
	ClearPending()
}

// SoftRestartResult is returned by soft restart + optional env reload.
type SoftRestartResult struct {
	CascLoaded bool   `json:"cascLoaded"`
	BuildName  string `json:"buildName,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ListfileEntry is a listfile search result row.
type ListfileEntry struct {
	FileDataID int    `json:"fileDataID"`
	FileName   string `json:"fileName"`
}

// Listfile provides listfile lookups.
type Listfile interface {
	IsLoaded() bool
	GetFilteredEntries(search string, useRegex bool) []ListfileEntry
	GetByID(fileDataID int) string
	GetByFilename(fileName string) int
	CollectBrowseFileIndex() (models, textures []ListfileEntry)
	CollectMapTileFileIndex() []ListfileEntry
}

// ModelSkin is a model skin descriptor.
type ModelSkin struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	DisplayID    int    `json:"displayID"`
	Textures     []int  `json:"textures"`
	ExtraGeosets []int  `json:"extraGeosets,omitempty"`
}

// ModelCache initializes and queries model skin caches.
type ModelCache interface {
	EnsureInitialized(ctx context.Context) error
	GetAllSkinsForModel(fileDataID int) ([]ModelSkin, error)
}

// MapEntry is a row from DBFilesClient/Map.db2.
type MapEntry struct {
	ID          int    `json:"id"`
	Name        any    `json:"name"`
	Dir         string `json:"dir"`
	ExpansionID any    `json:"expansionID"`
}

// MapList provides map metadata.
type MapList interface {
	GetMaps(ctx context.Context) ([]MapEntry, error)
}

// CharacterMetaParams mirrors CharacterMetaParams from TS.
type CharacterMetaParams struct {
	Race               int            `json:"race"`
	Gender             int            `json:"gender"`
	FileDataIDOverride *int           `json:"fileDataIdOverride,omitempty"`
	Customizations     map[string]int `json:"customizations"`
}

// CharacterMetaResponse is returned by CHAR_META.
type CharacterMetaResponse struct {
	FileDataID      int            `json:"fileDataID"`
	FileName        string         `json:"fileName"`
	TextureLayoutID int            `json:"textureLayoutID"`
	Choices         map[string]any `json:"choices"`
}

// CharacterMeta resolves character metadata.
type CharacterMeta interface {
	GetCharacterMeta(ctx context.Context, params CharacterMetaParams) (CharacterMetaResponse, error)
}

// ExportProgressSnapshot mirrors export progress state.
type ExportProgressSnapshot struct {
	CompletedSteps int        `json:"completedSteps"`
	TotalSteps     int        `json:"totalSteps"`
	TileIndex      int        `json:"tileIndex"`
	TileCount      int        `json:"tileCount"`
	StepsPerTile   int        `json:"stepsPerTile"`
	CurrentTile    *TileCoord `json:"currentTile,omitempty"`
	TaskName       string     `json:"taskName,omitempty"`
	TaskValue      int        `json:"taskValue,omitempty"`
	TaskMax        int        `json:"taskMax,omitempty"`
}

// TileCoord is an ADT tile coordinate pair.
type TileCoord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ExportProgress tracks batch export progress.
type ExportProgress interface {
	GetSnapshot(key string) (*ExportProgressSnapshot, bool)
	Finalize(key string) (*ExportProgressSnapshot, bool)
}

// ADTExportParams are POST /rest/exportADT body fields.
type ADTExportParams struct {
	MapID                  int    `json:"mapID"`
	MapDir                 string `json:"mapDir"`
	TileX                  int    `json:"tileX"`
	TileY                  int    `json:"tileY"`
	Quality                *int   `json:"quality,omitempty"`
	IncludeM2              *bool  `json:"includeM2,omitempty"`
	IncludeWMO             *bool  `json:"includeWMO,omitempty"`
	IncludeWMOSets         *bool  `json:"includeWMOSets,omitempty"`
	IncludeGameObjects     *bool  `json:"includeGameObjects,omitempty"`
	IncludeLiquid          *bool  `json:"includeLiquid,omitempty"`
	IncludeFoliage         *bool  `json:"includeFoliage,omitempty"`
	IncludeHoles           *bool  `json:"includeHoles,omitempty"`
	SplitAlphaMaps         *bool  `json:"splitAlphaMaps,omitempty"`
	SplitLargeTerrainBakes *bool  `json:"splitLargeTerrainBakes,omitempty"`
	GameObjects            []any  `json:"gameObjects,omitempty"`
	ProgressKey            string `json:"progressKey,omitempty"`
	TileIndex              *int   `json:"tileIndex,omitempty"`
	TileCount              *int   `json:"tileCount,omitempty"`
	StepsPerTile           *int   `json:"stepsPerTile,omitempty"`
	ExportAssetDir         string `json:"exportAssetDir,omitempty"`
}

// ADTExportResult is the EXPORT_RESULT payload for ADT exports.
type ADTExportResult struct {
	ExportID   int     `json:"exportID"`
	MapID      int     `json:"mapID"`
	MapDir     string  `json:"mapDir"`
	TileX      int     `json:"tileX"`
	TileY      int     `json:"tileY"`
	TileIndex  int     `json:"tileIndex"`
	ExportPath string  `json:"exportPath"`
	ExportType string  `json:"exportType"`
	MainFile   *string `json:"mainFile"`
}

// ADTExporter exports ADT terrain tiles.
type ADTExporter interface {
	Export(ctx context.Context, params ADTExportParams, exportID int) (ADTExportResult, error)
}

// MemoryDiagnostics mirrors debugMemory payload sections.
type MemoryDiagnostics struct {
	Summary      string           `json:"summary"`
	Process      map[string]int64 `json:"process"`
	Casc         map[string]any   `json:"casc"`
	Listfile     map[string]any   `json:"listfile"`
	Indexes      map[string]any   `json:"indexes"`
	DBCaches     map[string]any   `json:"dbCaches"`
	ExportCaches map[string]any   `json:"exportCaches"`
	Converter    map[string]any   `json:"converter"`
}

// MemoryDiagnosticsCollector collects memory debug info.
type MemoryDiagnosticsCollector interface {
	Collect() MemoryDiagnostics
}

// NoopLoader is a stub CASC loader used until real implementation is wired.
type NoopLoader struct{}

func (NoopLoader) IsLoaded() bool                    { return false }
func (NoopLoader) IsLoading() bool                   { return false }
func (NoopLoader) AwaitLoad(_ context.Context) error { return nil }
func (NoopLoader) LoadLocal(_ context.Context, _ string) ([]Build, error) {
	return nil, ErrNotImplemented
}
func (NoopLoader) LoadRemote(_ context.Context, _ string) ([]Build, error) {
	return nil, ErrNotImplemented
}
func (NoopLoader) LoadBuild(_ context.Context, _ int) (Source, error) {
	return nil, ErrNotImplemented
}
func (NoopLoader) Unload(_ context.Context) error { return nil }
func (NoopLoader) SoftRestart(_ context.Context, _ bool) (SoftRestartResult, error) {
	return SoftRestartResult{CascLoaded: false}, nil
}
func (NoopLoader) PendingBuilds() []Build                   { return nil }
func (NoopLoader) SetPendingFromLocal(_ string, _ []Build)  {}
func (NoopLoader) SetPendingFromRemote(_ string, _ []Build) {}
func (NoopLoader) ClearPending()                            {}

// NoopListfile is a stub listfile.
type NoopListfile struct{}

func (NoopListfile) IsLoaded() bool { return false }
func (NoopListfile) GetFilteredEntries(_ string, _ bool) []ListfileEntry {
	return []ListfileEntry{}
}
func (NoopListfile) GetByID(_ int) string       { return "" }
func (NoopListfile) GetByFilename(_ string) int { return 0 }
func (NoopListfile) CollectBrowseFileIndex() ([]ListfileEntry, []ListfileEntry) {
	return nil, nil
}
func (NoopListfile) CollectMapTileFileIndex() []ListfileEntry { return nil }

// NoopModelCache is a stub model cache.
type NoopModelCache struct{}

func (NoopModelCache) EnsureInitialized(_ context.Context) error { return nil }
func (NoopModelCache) GetAllSkinsForModel(_ int) ([]ModelSkin, error) {
	return []ModelSkin{}, nil
}

// NoopMapList is a stub map list provider.
type NoopMapList struct{}

func (NoopMapList) GetMaps(_ context.Context) ([]MapEntry, error) {
	return nil, ErrNotImplemented
}

// NoopCharacterMeta is a stub character meta provider.
type NoopCharacterMeta struct{}

func (NoopCharacterMeta) GetCharacterMeta(_ context.Context, _ CharacterMetaParams) (CharacterMetaResponse, error) {
	return CharacterMetaResponse{}, ErrNotImplemented
}

// NoopExportProgress is a stub export progress tracker.
type NoopExportProgress struct{}

func (NoopExportProgress) GetSnapshot(_ string) (*ExportProgressSnapshot, bool) {
	return nil, false
}
func (NoopExportProgress) Finalize(_ string) (*ExportProgressSnapshot, bool) {
	return nil, false
}

// NoopADTExporter is a stub ADT exporter.
type NoopADTExporter struct{}

func (NoopADTExporter) Export(_ context.Context, _ ADTExportParams, _ int) (ADTExportResult, error) {
	return ADTExportResult{}, ErrNotImplemented
}

// NoopMemoryDiagnostics is a stub memory diagnostics collector.
type NoopMemoryDiagnostics struct{}

func (NoopMemoryDiagnostics) Collect() MemoryDiagnostics {
	return MemoryDiagnostics{
		Summary:  "stub memory diagnostics",
		Process:  map[string]int64{},
		Casc:     map[string]any{"loaded": false},
		Listfile: map[string]any{},
		Indexes:  map[string]any{},
		DBCaches: map[string]any{},
		ExportCaches: map[string]any{},
		Converter: map[string]any{},
	}
}
