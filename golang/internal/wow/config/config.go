// Package config holds the mutable WoW reader configuration singleton.
package config

import "github.com/pqhuy98/wow-converter/internal/wow/constants"

// WowReaderConfig mirrors the TypeScript WowReaderConfig interface.
type WowReaderConfig struct {
	ListfileURL            string
	ListfileFallbackURL    string
	ListfileCacheRefresh   int
	DBDURL                 string
	DBDFallbackURL         string
	TactKeysURL            string
	TactKeysFallbackURL    string
	CacheExpiry            int
	CascLocale             int
	EnableUnknownFiles     bool
	CopyMode               string
	ListfileSortByID       bool
	ListfileShowFileDataIDs bool
	ExportDirectory        string
}

// WowConfig is the mutable singleton configuration.
var WowConfig = WowReaderConfig{
	ListfileURL:            "https://github.com/wowdev/wow-listfile/releases/latest/download/community-listfile.csv",
	ListfileFallbackURL:    "https://www.kruithne.net/wow.export/data/listfile/master",
	ListfileCacheRefresh:   3,
	DBDURL:                 "https://raw.githubusercontent.com/wowdev/WoWDBDefs/refs/heads/master/definitions/%s.dbd",
	DBDFallbackURL:         "https://www.kruithne.net/wow.export/data/dbd/?def=%s",
	TactKeysURL:            "https://raw.githubusercontent.com/wowdev/TACTKeys/master/WoW.txt",
	TactKeysFallbackURL:    "https://www.kruithne.net/wow.export/data/tact/wow",
	CacheExpiry:            7,
	CascLocale:             2,
	EnableUnknownFiles:     true,
	CopyMode:               "FULL",
	ListfileSortByID:       false,
	ListfileShowFileDataIDs: true,
	ExportDirectory:        constants.ExportPath,
}
