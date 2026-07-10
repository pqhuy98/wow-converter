package translators

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

// FilePath identifies a war3map file slot.
type FilePath string

const (
	FileInfo                  FilePath = "info"
	FileTerrain               FilePath = "terrain"
	FileUnits                 FilePath = "units"
	FileDoodads               FilePath = "doodads"
	FileCameras               FilePath = "cameras"
	FileRegions               FilePath = "regions"
	FileUnitData              FilePath = "unitData"
	FileUnitTypeSkins         FilePath = "unitTypeSkins"
	FileDestructibleTypeSkins FilePath = "destructibleTypeSkins"
	FileDoodadTypeSkins       FilePath = "doodadTypeSkins"
	FileAbilityTypeSkins      FilePath = "abilityTypeSkins"
	FileBuffTypeSkins         FilePath = "buffTypeSkins"
	FileItemData              FilePath = "itemData"
	FileDestructibleData      FilePath = "destructibleData"
	FileDoodadData             FilePath = "doodadData"
	FileAbilityData           FilePath = "abilityData"
	FileBuffData              FilePath = "buffData"
	FileUpgradeData           FilePath = "upgradeData"
)

// MapTranslator loads and saves WC3 map binary files.
type MapTranslator struct {
	Info                  data.Info
	Terrain               data.Terrain
	Units                 []data.Unit
	Doodads               []data.Doodad
	SpecialDoodads        []data.SpecialDoodad
	Cameras               []data.Camera
	Regions               []data.Region
	UnitData              data.ObjectModificationTable
	DestructibleData      data.ObjectModificationTable
	DoodadData            data.ObjectModificationTable
	ItemData              data.ObjectModificationTable
	AbilityData           data.ObjectModificationTable
	BuffData              data.ObjectModificationTable
	UpgradeData           data.ObjectModificationTable
	UnitTypeSkins         data.ObjectModificationTable
	DestructibleTypeSkins data.ObjectModificationTable
	DoodadTypeSkins       data.ObjectModificationTable
	AbilityTypeSkins      data.ObjectModificationTable
	BuffTypeSkins         data.ObjectModificationTable
	filePaths             map[FilePath]string
}

// NewMapTranslator returns an empty map translator.
func NewMapTranslator() *MapTranslator {
	return &MapTranslator{
		UnitData:              data.NewObjectModificationTable(),
		DestructibleData:      data.NewObjectModificationTable(),
		DoodadData:            data.NewObjectModificationTable(),
		ItemData:              data.NewObjectModificationTable(),
		AbilityData:           data.NewObjectModificationTable(),
		BuffData:              data.NewObjectModificationTable(),
		UpgradeData:           data.NewObjectModificationTable(),
		UnitTypeSkins:         data.NewObjectModificationTable(),
		DestructibleTypeSkins: data.NewObjectModificationTable(),
		DoodadTypeSkins:       data.NewObjectModificationTable(),
		AbilityTypeSkins:      data.NewObjectModificationTable(),
		BuffTypeSkins:         data.NewObjectModificationTable(),
	}
}

// SetMapDir configures output paths for map files.
func (m *MapTranslator) SetMapDir(mapDir string) {
	m.filePaths = map[FilePath]string{
		FileInfo:                  filepath.Join(mapDir, "war3map.w3i"),
		FileTerrain:               filepath.Join(mapDir, "war3map.w3e"),
		FileUnits:                 filepath.Join(mapDir, "war3mapUnits.doo"),
		FileDoodads:               filepath.Join(mapDir, "war3map.doo"),
		FileCameras:               filepath.Join(mapDir, "war3map.w3c"),
		FileRegions:               filepath.Join(mapDir, "war3map.w3r"),
		FileUnitData:              filepath.Join(mapDir, "war3map.w3u"),
		FileDestructibleData:      filepath.Join(mapDir, "war3map.w3b"),
		FileDoodadData:            filepath.Join(mapDir, "war3map.w3d"),
		FileItemData:              filepath.Join(mapDir, "war3map.w3t"),
		FileAbilityData:           filepath.Join(mapDir, "war3map.w3a"),
		FileBuffData:              filepath.Join(mapDir, "war3map.w3h"),
		FileUpgradeData:           filepath.Join(mapDir, "war3map.w3q"),
		FileUnitTypeSkins:         filepath.Join(mapDir, "war3mapSkin.w3u"),
		FileDestructibleTypeSkins: filepath.Join(mapDir, "war3mapSkin.w3b"),
		FileDoodadTypeSkins:       filepath.Join(mapDir, "war3mapSkin.w3d"),
		FileAbilityTypeSkins:     filepath.Join(mapDir, "war3mapSkin.w3a"),
		FileBuffTypeSkins:         filepath.Join(mapDir, "war3mapSkin.w3h"),
	}
}

// Load reads map files from mapDir.
func (m *MapTranslator) Load(mapDir string) error {
	m.SetMapDir(mapDir)

	infoBytes, err := os.ReadFile(m.filePaths[FileInfo])
	if err != nil {
		return fmt.Errorf("load map %s: %w", mapDir, err)
	}
	m.Info = InfoTranslator{}.WarToJSON(infoBytes).JSON

	terrainBytes, err := os.ReadFile(m.filePaths[FileTerrain])
	if err != nil {
		return fmt.Errorf("load terrain: %w", err)
	}
	m.Terrain = TerrainTranslator{}.WarToJSON(terrainBytes).JSON

	unitsBytes, err := os.ReadFile(m.filePaths[FileUnits])
	if err != nil {
		return fmt.Errorf("load units: %w", err)
	}
	m.Units = UnitsTranslator{}.WarToJSON(unitsBytes).JSON

	doodadsBytes, err := os.ReadFile(m.filePaths[FileDoodads])
	if err != nil {
		return fmt.Errorf("load doodads: %w", err)
	}
	allDoodads := DoodadsTranslator{}.WarToJSON(doodadsBytes).JSON
	m.Doodads = allDoodads.Doodads
	m.SpecialDoodads = allDoodads.SpecialDoodads

	if _, err := os.Stat(m.filePaths[FileCameras]); err == nil {
		camerasBytes, readErr := os.ReadFile(m.filePaths[FileCameras])
		if readErr != nil {
			return fmt.Errorf("load cameras: %w", readErr)
		}
		m.Cameras = CamerasTranslator{}.WarToJSON(camerasBytes).JSON
	}

	if _, err := os.Stat(m.filePaths[FileRegions]); err == nil {
		regionsBytes, readErr := os.ReadFile(m.filePaths[FileRegions])
		if readErr != nil {
			return fmt.Errorf("load regions: %w", readErr)
		}
		m.Regions = RegionsTranslator{}.WarToJSON(regionsBytes).JSON
	}

	m.loadObjectFile(FileUnitData, data.ObjectUnits, &m.UnitData)
	m.loadObjectFile(FileUnitTypeSkins, data.ObjectUnits, &m.UnitTypeSkins)
	m.loadObjectFile(FileDestructibleTypeSkins, data.ObjectDestructables, &m.DestructibleTypeSkins)
	m.loadObjectFile(FileDoodadTypeSkins, data.ObjectDoodads, &m.DoodadTypeSkins)
	m.loadObjectFile(FileAbilityTypeSkins, data.ObjectAbilities, &m.AbilityTypeSkins)
	m.loadObjectFile(FileBuffTypeSkins, data.ObjectBuffs, &m.BuffTypeSkins)
	m.loadObjectFile(FileItemData, data.ObjectItems, &m.ItemData)
	m.loadObjectFile(FileDestructibleData, data.ObjectDestructables, &m.DestructibleData)
	m.loadObjectFile(FileDoodadData, data.ObjectDoodads, &m.DoodadData)
	m.loadObjectFile(FileAbilityData, data.ObjectAbilities, &m.AbilityData)
	m.loadObjectFile(FileBuffData, data.ObjectBuffs, &m.BuffData)
	m.loadObjectFile(FileUpgradeData, data.ObjectUpgrades, &m.UpgradeData)

	return nil
}

func (m *MapTranslator) loadObjectFile(file FilePath, objectType data.ObjectType, dest *data.ObjectModificationTable) {
	if _, err := os.Stat(m.filePaths[file]); err != nil {
		return
	}
	bytes, err := os.ReadFile(m.filePaths[file])
	if err != nil {
		return
	}
	*dest = WarToJSONObjects(objectType, bytes).JSON
}

// Save writes a single map file.
func (m *MapTranslator) Save(file FilePath) error {
	path, ok := m.filePaths[file]
	if !ok || path == "" {
		return fmt.Errorf("map dir not set")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var out []byte
	switch file {
	case FileInfo:
		out = InfoTranslator{}.JSONToWar(m.Info).Buffer
	case FileTerrain:
		out = TerrainTranslator{}.JSONToWar(m.Terrain).Buffer
	case FileUnits:
		out = UnitsTranslator{}.JSONToWar(m.Units).Buffer
	case FileDoodads:
		out = DoodadsTranslator{}.JSONToWar(data.DoodadList{
			Doodads: m.Doodads, SpecialDoodads: m.SpecialDoodads,
		}).Buffer
	case FileCameras:
		out = CamerasTranslator{}.JSONToWar(m.Cameras).Buffer
	case FileRegions:
		out = RegionsTranslator{}.JSONToWar(m.Regions).Buffer
	case FileUnitData:
		out = JSONToWarObjects(data.ObjectUnits, m.UnitData).Buffer
	case FileUnitTypeSkins:
		out = JSONToWarObjects(data.ObjectUnits, m.UnitTypeSkins).Buffer
	case FileDestructibleTypeSkins:
		out = JSONToWarObjects(data.ObjectDestructables, m.DestructibleTypeSkins).Buffer
	case FileDoodadTypeSkins:
		out = JSONToWarObjects(data.ObjectDoodads, m.DoodadTypeSkins).Buffer
	case FileAbilityTypeSkins:
		out = JSONToWarObjects(data.ObjectAbilities, m.AbilityTypeSkins).Buffer
	case FileBuffTypeSkins:
		out = JSONToWarObjects(data.ObjectBuffs, m.BuffTypeSkins).Buffer
	case FileItemData:
		out = JSONToWarObjects(data.ObjectItems, m.ItemData).Buffer
	case FileDestructibleData:
		out = JSONToWarObjects(data.ObjectDestructables, m.DestructibleData).Buffer
	case FileDoodadData:
		out = JSONToWarObjects(data.ObjectDoodads, m.DoodadData).Buffer
	case FileAbilityData:
		out = JSONToWarObjects(data.ObjectAbilities, m.AbilityData).Buffer
	case FileBuffData:
		out = JSONToWarObjects(data.ObjectBuffs, m.BuffData).Buffer
	case FileUpgradeData:
		out = JSONToWarObjects(data.ObjectUpgrades, m.UpgradeData).Buffer
	default:
		return fmt.Errorf("unknown file path %s", file)
	}
	return os.WriteFile(path, out, 0o644)
}
