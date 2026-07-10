package server

import (
	"os"
	"strconv"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// ReaderConfig mirrors src/lib/wow/server/config.ts WowReaderConfig.
type ReaderConfig struct {
	// Remote data sources
	ListfileURL          string `json:"listfileURL"`
	ListfileFallbackURL  string `json:"listfileFallbackURL"`
	ListfileCacheRefresh int    `json:"listfileCacheRefresh"`
	DBDURL               string `json:"dbdURL"`
	DBDFallbackURL       string `json:"dbdFallbackURL"`
	TactKeysURL          string `json:"tactKeysURL"`
	TactKeysFallbackURL  string `json:"tactKeysFallbackURL"`
	CacheExpiry          int    `json:"cacheExpiry"`

	// CASC
	CascLocale         int  `json:"cascLocale"`
	EnableUnknownFiles bool `json:"enableUnknownFiles"`

	// Export shaping
	CopyMode                   string `json:"copyMode"`
	ListfileSortByID           bool   `json:"listfileSortByID"`
	ListfileShowFileDataIDs    bool   `json:"listfileShowFileDataIDs"`
	EnableM2Skins              bool   `json:"enableM2Skins"`
	EnableSharedTextures       bool   `json:"enableSharedTextures"`
	EnableSharedChildren       bool   `json:"enableSharedChildren"`
	EnableAbsoluteMTLPaths     bool   `json:"enableAbsoluteMTLPaths"`
	EnableAbsoluteCSVPaths     bool   `json:"enableAbsoluteCSVPaths"`
	RemovePathSpaces           bool   `json:"removePathSpaces"`
	RemovePathSpacesCopy       bool   `json:"removePathSpacesCopy"`
	ExportTextureFormat        string `json:"exportTextureFormat"`
	ExportModelFormat          string `json:"exportModelFormat"`
	MaxTextureSize             int    `json:"maxTextureSize"`
	ExportChannelMask          int    `json:"exportChannelMask"`
	ExportM2Bones              bool   `json:"exportM2Bones"`
	ExportM2Meta               bool   `json:"exportM2Meta"`
	ExportWMOMeta              bool   `json:"exportWMOMeta"`
	ExportBLPMeta              bool   `json:"exportBLPMeta"`
	ExportFoliageMeta          bool   `json:"exportFoliageMeta"`
	ExportNamedFiles           bool   `json:"exportNamedFiles"`
	OverwriteFiles             bool   `json:"overwriteFiles"`
	ModelsExportSkin           bool   `json:"modelsExportSkin"`
	ModelsExportSkel           bool   `json:"modelsExportSkel"`
	ModelsExportBone           bool   `json:"modelsExportBone"`
	ModelsExportAnim           bool   `json:"modelsExportAnim"`
	ModelsExportWMOGroups      bool   `json:"modelsExportWMOGroups"`
	ModelsExportUV2            bool   `json:"modelsExportUV2"`
	ModelsExportTextures       bool   `json:"modelsExportTextures"`
	ModelsExportAlpha          bool   `json:"modelsExportAlpha"`
	ModelsExportAnimations     bool   `json:"modelsExportAnimations"`
	ModelsExportCollision      bool   `json:"modelsExportCollision"`
	ModelsExportWithBonePrefix bool   `json:"modelsExportWithBonePrefix"`
	ModelsExportPngIncrements  bool   `json:"modelsExportPngIncrements"`

	// Maps / ADT
	MapsIncludeWMO         bool `json:"mapsIncludeWMO"`
	MapsIncludeM2          bool `json:"mapsIncludeM2"`
	MapsIncludeWMOSets     bool `json:"mapsIncludeWMOSets"`
	MapsIncludeFoliage     bool `json:"mapsIncludeFoliage"`
	MapsIncludeLiquid      bool `json:"mapsIncludeLiquid"`
	MapsIncludeGameObjects bool `json:"mapsIncludeGameObjects"`
	MapsIncludeHoles       bool `json:"mapsIncludeHoles"`
	ExportMapQuality       int  `json:"exportMapQuality"`
	SplitLargeTerrainBakes bool `json:"splitLargeTerrainBakes"`
	SplitAlphaMaps         bool `json:"splitAlphaMaps"`

	// Character
	ChrIncludeBaseClothing bool `json:"chrIncludeBaseClothing"`

	PathFormat      string `json:"pathFormat"`
	ExportDirectory string `json:"exportDirectory"`
}

var (
	configMu sync.RWMutex
	// WowConfig is the mutable singleton config (analogous to wowConfig in TS).
	WowConfig = DefaultConfig()
)

func defaultExportPath() string {
	return workspace.DefaultExportDir()
}

// DefaultConfig returns defaults matching src/lib/wow/server/config.ts.
func DefaultConfig() *ReaderConfig {
	return &ReaderConfig{
		ListfileURL:          "https://github.com/wowdev/wow-listfile/releases/latest/download/community-listfile.csv",
		ListfileFallbackURL:  "https://www.kruithne.net/wow.export/data/listfile/master",
		ListfileCacheRefresh: 3,
		DBDURL:               "https://raw.githubusercontent.com/wowdev/WoWDBDefs/refs/heads/master/definitions/%s.dbd",
		DBDFallbackURL:       "https://www.kruithne.net/wow.export/data/dbd/?def=%s",
		TactKeysURL:          "https://raw.githubusercontent.com/wowdev/TACTKeys/master/WoW.txt",
		TactKeysFallbackURL:  "https://www.kruithne.net/wow.export/data/tact/wow",
		CacheExpiry:          7,

		CascLocale:         2,
		EnableUnknownFiles: true,

		CopyMode:                   "FULL",
		ListfileSortByID:           false,
		ListfileShowFileDataIDs:    true,
		EnableM2Skins:              true,
		EnableSharedTextures:       true,
		EnableSharedChildren:       true,
		EnableAbsoluteMTLPaths:     false,
		EnableAbsoluteCSVPaths:     false,
		RemovePathSpaces:           true,
		RemovePathSpacesCopy:       true,
		ExportTextureFormat:        "PNG",
		ExportModelFormat:          "OBJ",
		MaxTextureSize:             512,
		ExportChannelMask:          15,
		ExportM2Bones:              true,
		ExportM2Meta:               true,
		ExportWMOMeta:              true,
		ExportBLPMeta:              false,
		ExportFoliageMeta:          false,
		ExportNamedFiles:           true,
		OverwriteFiles:             true,
		ModelsExportSkin:           true,
		ModelsExportSkel:           true,
		ModelsExportBone:           true,
		ModelsExportAnim:           true,
		ModelsExportWMOGroups:      true,
		ModelsExportUV2:            true,
		ModelsExportTextures:       true,
		ModelsExportAlpha:          true,
		ModelsExportAnimations:     true,
		ModelsExportCollision:      true,
		ModelsExportWithBonePrefix: true,
		ModelsExportPngIncrements:  true,

		MapsIncludeWMO:         true,
		MapsIncludeM2:          true,
		MapsIncludeWMOSets:     true,
		MapsIncludeFoliage:     true,
		MapsIncludeLiquid:      false,
		MapsIncludeGameObjects: true,
		MapsIncludeHoles:       true,
		ExportMapQuality:       4096,
		SplitLargeTerrainBakes: true,
		SplitAlphaMaps:         true,

		ChrIncludeBaseClothing: true,

		PathFormat:      "win32",
		ExportDirectory: defaultExportPath(),
	}
}

// GetConfig returns a snapshot of the current config.
func GetConfig() ReaderConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	return *WowConfig
}

// SetConfigValue sets a config field by key (used by /rest/setConfig).
func SetConfigValue(key string, value any) bool {
	if !IsSettableConfigKey(key) {
		return false
	}
	configMu.Lock()
	defer configMu.Unlock()
	return setConfigField(WowConfig, key, value)
}

// GetConfigValue returns a single config field by key.
func GetConfigValue(key string) (any, bool) {
	configMu.RLock()
	defer configMu.RUnlock()
	return getConfigField(WowConfig, key)
}

// GetServerPort reads WOW_DATA_SERVER_PORT (default 17753).
func GetServerPort() int {
	const defaultPort = 17753
	if v := os.Getenv("WOW_DATA_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			return port
		}
	}
	return defaultPort
}
