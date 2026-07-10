package extra

import (
	"strings"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/translators"
)

func stripMapExtension(name string) string {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".w3x") || strings.HasSuffix(lower, ".w3m") {
		return name[:len(name)-4]
	}
	return name
}

// ObjectData is shared unit/doodad/ability object-type metadata.
type ObjectData struct {
	Code   string
	Parent string
	Data   []data.Modification
}

// UnitType is a custom unit object definition.
type UnitType struct {
	ObjectData
}

// DoodadType is a custom doodad or destructible definition.
type DoodadType struct {
	ObjectData
	IsDestructible bool
}

// UnitInstance is a placed unit with resolved type.
type UnitInstance struct {
	data.Unit
	Type any // UnitType or string
}

// DoodadInstance is a placed doodad with resolved type.
type DoodadInstance struct {
	data.Doodad
	Type any // DoodadType or string
}

// MapManager is the high-level WC3 map editor state.
type MapManager struct {
	MapData               *translators.MapTranslator
	FourCCGenerator       *FourCCGenerator
	UnitTypes             []UnitType
	DoodadTypes           []DoodadType
	DestructibleTypes     []DoodadType
	Units                 []UnitInstance
	Doodads               []DoodadInstance
	Abilities             []ObjectData
	BuffTypes             []ObjectData
	Regions               []data.Region
	Cameras               []data.Camera
	Info                  data.Info
	Players               []data.Player
	UnitTypeSkins         data.ObjectModificationTable
	DestructibleTypeSkins data.ObjectModificationTable
	DoodadTypeSkins       data.ObjectModificationTable
	AbilityTypeSkins      data.ObjectModificationTable
	BuffTypeSkins         data.ObjectModificationTable
}

// NewMapManager creates an empty map manager.
func NewMapManager() *MapManager {
	return &MapManager{
		MapData:               translators.NewMapTranslator(),
		FourCCGenerator:       NewFourCCGenerator(),
		UnitTypeSkins:         data.NewObjectModificationTable(),
		DestructibleTypeSkins: data.NewObjectModificationTable(),
		DoodadTypeSkins:       data.NewObjectModificationTable(),
		AbilityTypeSkins:      data.NewObjectModificationTable(),
		BuffTypeSkins:         data.NewObjectModificationTable(),
	}
}

func (m *MapManager) registerTableFourCCs(table data.ObjectModificationTable) {
	for key := range table.Original {
		if len(key) >= 4 {
			m.FourCCGenerator.AddUsed(key[:4])
		}
	}
	for key := range table.Custom {
		if len(key) >= 4 {
			m.FourCCGenerator.AddUsed(key[:4+1-1])
		}
	}
}

// Load reads an existing WC3 map directory.
func (m *MapManager) Load(mapDir string) error {
	if err := m.MapData.Load(mapDir); err != nil {
		return err
	}
	m.Info = m.MapData.Info
	m.Players = m.Info.Players

	m.registerTableFourCCs(m.MapData.UnitData)
	m.registerTableFourCCs(m.MapData.ItemData)
	m.registerTableFourCCs(m.MapData.DestructibleData)
	m.registerTableFourCCs(m.MapData.DoodadData)
	m.registerTableFourCCs(m.MapData.AbilityData)
	m.registerTableFourCCs(m.MapData.BuffData)
	m.registerTableFourCCs(m.MapData.UpgradeData)

	m.UnitTypeSkins = m.MapData.UnitTypeSkins
	m.DestructibleTypeSkins = m.MapData.DestructibleTypeSkins
	m.DoodadTypeSkins = m.MapData.DoodadTypeSkins
	m.AbilityTypeSkins = m.MapData.AbilityTypeSkins
	m.BuffTypeSkins = m.MapData.BuffTypeSkins

	for key, mods := range m.MapData.UnitData.Custom {
		code, parent := SplitObjectKey(key)
		m.UnitTypes = append(m.UnitTypes, UnitType{ObjectData: ObjectData{Code: code, Parent: parent, Data: mods}})
	}
	for key, mods := range m.MapData.DoodadData.Custom {
		code, parent := SplitObjectKey(key)
		m.DoodadTypes = append(m.DoodadTypes, DoodadType{ObjectData: ObjectData{Code: code, Parent: parent, Data: mods}})
	}
	for key, mods := range m.MapData.DestructibleData.Custom {
		code, parent := SplitObjectKey(key)
		m.DestructibleTypes = append(m.DestructibleTypes, DoodadType{
			ObjectData: ObjectData{Code: code, Parent: parent, Data: mods}, IsDestructible: true,
		})
	}

	for _, unit := range m.MapData.Units {
		inst := UnitInstance{Unit: unit, Type: unit.Type}
		for i := range m.UnitTypes {
			if m.UnitTypes[i].Code == unit.Type {
				inst.Type = &m.UnitTypes[i]
				break
			}
		}
		m.Units = append(m.Units, inst)
	}
	for _, doodad := range m.MapData.Doodads {
		inst := DoodadInstance{Doodad: doodad, Type: doodad.Type}
		for i := range m.DoodadTypes {
			if m.DoodadTypes[i].Code == doodad.Type {
				inst.Type = &m.DoodadTypes[i]
				break
			}
		}
		m.Doodads = append(m.Doodads, inst)
	}
	for key, mods := range m.MapData.AbilityData.Custom {
		code, parent := SplitObjectKey(key)
		m.Abilities = append(m.Abilities, ObjectData{Code: code, Parent: parent, Data: mods})
	}
	for key, mods := range m.MapData.BuffData.Custom {
		code, parent := SplitObjectKey(key)
		m.BuffTypes = append(m.BuffTypes, ObjectData{Code: code, Parent: parent, Data: mods})
	}

	m.Regions = m.MapData.Regions
	m.Cameras = m.MapData.Cameras
	return nil
}

// Terrain returns the current terrain.
func (m *MapManager) GetTerrain() data.Terrain {
	return m.MapData.Terrain
}

// SetTerrain updates terrain data.
func (m *MapManager) SetTerrain(terrain data.Terrain) {
	m.MapData.Terrain = terrain
}

// AddUnitType creates a custom unit type and returns a pointer to the stored entry.
func (m *MapManager) AddUnitType(isHero bool, parentCode string, mods []data.Modification) *UnitType {
	prefix := "lower"
	if isHero {
		prefix = "upper"
	}
	gen, _ := m.FourCCGenerator.Generate(prefix)
	ut := UnitType{ObjectData: ObjectData{Code: gen.CodeString, Parent: parentCode, Data: mods}}
	m.UnitTypes = append(m.UnitTypes, ut)
	return &m.UnitTypes[len(m.UnitTypes)-1]
}

// AddDoodadType creates a custom doodad or destructible type and returns a pointer to the stored entry.
func (m *MapManager) AddDoodadType(mods []data.Modification, isDestructible bool) *DoodadType {
	gen, _ := m.FourCCGenerator.Generate("any")
	parent := BaseDoodadType
	if isDestructible {
		parent = BaseDestructibleType
	}
	dt := DoodadType{ObjectData: ObjectData{Code: gen.CodeString, Parent: parent, Data: mods}, IsDestructible: isDestructible}
	if isDestructible {
		m.DestructibleTypes = append(m.DestructibleTypes, dt)
		return &m.DestructibleTypes[len(m.DestructibleTypes)-1]
	}
	m.DoodadTypes = append(m.DoodadTypes, dt)
	return &m.DoodadTypes[len(m.DoodadTypes)-1]
}

// AddUnit places a unit instance.
func (m *MapManager) AddUnit(unitType *UnitType, unit data.Unit) *UnitInstance {
	inst := UnitInstance{Unit: unit, Type: unitType}
	m.Units = append(m.Units, inst)
	return &m.Units[len(m.Units)-1]
}

// AddDoodad places a doodad instance.
func (m *MapManager) AddDoodad(doodadType *DoodadType, doodad data.Doodad) *DoodadInstance {
	inst := DoodadInstance{Doodad: doodad, Type: doodadType}
	m.Doodads = append(m.Doodads, inst)
	return &m.Doodads[len(m.Doodads)-1]
}

// AddAbility creates a custom ability type.
func (m *MapManager) AddAbility(parentCode string, mods []data.Modification) *ObjectData {
	gen, _ := m.FourCCGenerator.Generate("any")
	ab := ObjectData{Code: gen.CodeString, Parent: parentCode, Data: mods}
	m.Abilities = append(m.Abilities, ab)
	return &m.Abilities[len(m.Abilities)-1]
}

// EnsureMapInfo fills war3map.w3i metadata when missing (required by World Editor).
// Defaults mirror HiveWE MapInfo::load_defaults() + update_map_bounds_info().
func (m *MapManager) EnsureMapInfo(mapName string) {
	if m.Info.FileVersion == 0 {
		m.Info = translators.DefaultInfo()
	}
	t := m.MapData.Terrain
	if tileset := strings.TrimSpace(t.Tileset); tileset != "" {
		m.Info.Map.MainTileType = tileset[:1]
	}
	if t.Map.Width > 0 && t.Map.Height > 0 {
		updateMapBoundsFromTerrain(&m.Info, t)
	}
	if mapName != "" && m.Info.Map.Name == "" {
		m.Info.Map.Name = stripMapExtension(mapName)
	}
	if len(m.Info.Players) == 0 {
		m.Info.Players = translators.DefaultInfo().Players
	}
	if len(m.Info.Forces) == 0 {
		m.Info.Forces = translators.DefaultInfo().Forces
	}
}

func updateMapBoundsFromTerrain(info *data.Info, terrain data.Terrain) {
	const tileSize = 128
	unplayableLeft, unplayableRight := 0, 0
	unplayableBottom, unplayableTop := 0, 0

	terrainWidth := terrain.Map.Width + 1
	terrainHeight := terrain.Map.Height + 1
	offsetX := terrain.Map.Offset.X
	offsetY := terrain.Map.Offset.Y

	info.Camera.Complements = [4]int32{
		int32(unplayableLeft),
		int32(unplayableRight),
		int32(unplayableBottom),
		int32(unplayableTop),
	}
	info.Map.PlayableArea.Width = terrainWidth - 1 - unplayableLeft - unplayableRight
	info.Map.PlayableArea.Height = terrainHeight - 1 - unplayableBottom - unplayableTop

	leftBottomX := float32((unplayableLeft+4)*tileSize) + offsetX
	leftBottomY := float32((unplayableBottom+2)*tileSize) + offsetY
	rightTopX := float32((terrainWidth-1-unplayableRight-4)*tileSize) + offsetX
	rightTopY := float32((terrainHeight-1-unplayableTop-2)*tileSize) + offsetY

	info.Camera.Bounds = [8]float32{
		leftBottomX, leftBottomY,
		rightTopX, rightTopY,
		leftBottomX, rightTopY,
		rightTopX, leftBottomY,
	}
}

// Save writes all map files to mapDir.
func (m *MapManager) Save(mapDir string) error {
	m.MapData.UnitData.Custom = map[string][]data.Modification{}
	for _, ut := range m.UnitTypes {
		m.MapData.UnitData.Custom[ut.Code+":"+ut.Parent] = ut.Data
	}
	m.MapData.DoodadData.Custom = map[string][]data.Modification{}
	for _, dt := range m.DoodadTypes {
		m.MapData.DoodadData.Custom[dt.Code+":"+dt.Parent] = dt.Data
	}
	m.MapData.DestructibleData.Custom = map[string][]data.Modification{}
	for _, dt := range m.DestructibleTypes {
		m.MapData.DestructibleData.Custom[dt.Code+":"+dt.Parent] = dt.Data
	}

	m.MapData.Units = nil
	for _, u := range m.Units {
		unit := u.Unit
		unit.Type = typeCode(u.Type)
		m.MapData.Units = append(m.MapData.Units, unit)
	}
	m.MapData.Doodads = nil
	for _, d := range m.Doodads {
		doodad := d.Doodad
		doodad.Type = typeCode(d.Type)
		m.MapData.Doodads = append(m.MapData.Doodads, doodad)
	}

	m.MapData.AbilityData.Custom = map[string][]data.Modification{}
	for _, ab := range m.Abilities {
		m.MapData.AbilityData.Custom[ab.Code+":"+ab.Parent] = ab.Data
	}

	m.MapData.Regions = m.Regions
	m.MapData.Cameras = m.Cameras
	if m.Players != nil {
		m.Info.Players = m.Players
	}
	m.MapData.Info = m.Info

	m.EnsureMapInfo("")
	m.MapData.SetMapDir(mapDir)
	if err := m.MapData.Save(translators.FileInfo); err != nil {
		return err
	}
	files := []translators.FilePath{
		translators.FileUnits, translators.FileDoodads,
		translators.FileTerrain, translators.FileCameras, translators.FileRegions,
		translators.FileUnitData, translators.FileDoodadData, translators.FileDestructibleData,
		translators.FileAbilityData,
	}
	for _, f := range files {
		if err := m.MapData.Save(f); err != nil {
			return err
		}
	}
	return nil
}

// FindDoodadTypeWithString finds a doodad type by string modification field.
func (m *MapManager) FindDoodadTypeWithString(fieldID string, predicate func(string) bool) *DoodadType {
	for i := range m.DoodadTypes {
		dt := &m.DoodadTypes[i]
		for _, mod := range dt.Data {
			if mod.ID == fieldID && mod.Type == data.ModificationString {
				if s, ok := mod.Value.(string); ok && predicate(s) {
					return dt
				}
			}
		}
	}
	return nil
}

// FindDoodadWithType finds a doodad instance by type.
func (m *MapManager) FindDoodadWithType(doodadType any) *DoodadInstance {
	target := typeCode(doodadType)
	for i := range m.Doodads {
		if typeCode(m.Doodads[i].Type) == target {
			return &m.Doodads[i]
		}
	}
	return nil
}

func typeCode(t any) string {
	switch v := t.(type) {
	case string:
		return v
	case UnitType:
		return v.Code
	case *UnitType:
		if v == nil {
			return ""
		}
		return v.Code
	case DoodadType:
		return v.Code
	case *DoodadType:
		if v == nil {
			return ""
		}
		return v.Code
	default:
		return ""
	}
}

// ModStringValue finds a string modification by id.
func ModStringValue(mods []data.Modification, id string) (string, bool) {
	for _, m := range mods {
		if m.ID == id && m.Type == data.ModificationString {
			if s, ok := m.Value.(string); ok {
				return strings.ReplaceAll(s, "\\", "/"), true
			}
		}
	}
	return "", false
}
