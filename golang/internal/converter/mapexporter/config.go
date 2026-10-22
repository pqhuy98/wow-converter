package mapexporter

import "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"

type MapExportConfig = settings.MapExportConfig
type TerrainClampPercent = settings.TerrainClampPercent
type DoodadEnable = settings.DoodadEnable
type NavigationConfig = settings.NavigationConfig
type TileCoord = settings.TileCoord

func DefaultMapExportConfig() MapExportConfig   { return settings.DefaultMapExportConfig() }
func DefaultNavigationConfig() NavigationConfig { return settings.DefaultNavigationConfig() }
