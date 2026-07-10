package translators

import (
	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

var infoTranslatorInstance = &InfoTranslator{}

// InfoTranslator handles war3map.w3i.
type InfoTranslator struct{}

// GetInfoTranslator returns the singleton InfoTranslator.
func GetInfoTranslator() *InfoTranslator {
	return infoTranslatorInstance
}

// JSONToWar serializes Info to binary.
func (InfoTranslator) JSONToWar(info data.Info) wc3.WarResult {
	return infoTranslatorInstance.jsonToWar(info)
}

// WarToJSON parses war3map.w3i bytes into Info.
func (InfoTranslator) WarToJSON(buffer []byte) wc3.JsonResult[data.Info] {
	return infoTranslatorInstance.warToJSON(buffer)
}

func (*InfoTranslator) jsonToWar(infoJson data.Info) wc3.WarResult {
	out := wc3.NewHexBufferWriter()

	out.AddInt(int(infoJson.FileVersion))
	saves := infoJson.Saves
	out.AddInt(int(saves))
	out.AddInt(int(infoJson.EditorVersion))

	out.AddInt(infoJson.GameVersion.Major)
	out.AddInt(infoJson.GameVersion.Minor)
	out.AddInt(infoJson.GameVersion.Patch)
	out.AddInt(infoJson.GameVersion.Build)

	out.AddString(infoJson.Map.Name)
	out.AddString(infoJson.Map.Author)
	out.AddString(infoJson.Map.Description)
	out.AddString(infoJson.Map.RecommendedPlayers)

	for i := 0; i < 8; i++ {
		out.AddFloat(infoJson.Camera.Bounds[i])
	}
	for i := 0; i < 4; i++ {
		out.AddInt(int(infoJson.Camera.Complements[i]))
	}

	out.AddInt(infoJson.Map.PlayableArea.Width)
	out.AddInt(infoJson.Map.PlayableArea.Height)

	flags := mapFlagsToInt(infoJson.Map.Flags)
	out.AddInt(flags)
	out.AddChar(infoJson.Map.MainTileType)

	out.AddInt(infoJson.LoadingScreen.Background)
	out.AddString(infoJson.LoadingScreen.Path)
	out.AddString(infoJson.LoadingScreen.Text)
	out.AddString(infoJson.LoadingScreen.Title)
	out.AddString(infoJson.LoadingScreen.Subtitle)

	out.AddInt(int(infoJson.GameDataSet))

	out.AddString(infoJson.Prologue.Path)
	out.AddString(infoJson.Prologue.Text)
	out.AddString(infoJson.Prologue.Title)
	out.AddString(infoJson.Prologue.Subtitle)

	out.AddInt(infoJson.Fog.Type)
	out.AddFloat(infoJson.Fog.StartHeight)
	out.AddFloat(infoJson.Fog.EndHeight)
	out.AddFloat(infoJson.Fog.Density)
	out.AddByte(infoJson.Fog.Color[0])
	out.AddByte(infoJson.Fog.Color[1])
	out.AddByte(infoJson.Fog.Color[2])
	out.AddByte(infoJson.Fog.Color[3])

	out.AddInt(int(infoJson.GlobalWeather))
	out.AddString(infoJson.CustomSoundEnv)
	out.AddByte(infoJson.CustomLightEnv)

	out.AddByte(infoJson.Water[0])
	out.AddByte(infoJson.Water[1])
	out.AddByte(infoJson.Water[2])
	out.AddByte(infoJson.Water[3])

	out.AddInt(int(infoJson.ScriptLanguage))
	out.AddInt(int(infoJson.SupportedModes))
	out.AddInt(int(infoJson.GameDataVersion))

	out.AddInt(int(infoJson.DefaultCameraZoom))
	out.AddInt(int(infoJson.MaxCameraZoom))
	out.AddInt(int(infoJson.MinCameraZoom))

	out.AddInt(len(infoJson.Players))
	for _, player := range infoJson.Players {
		out.AddInt(player.PlayerNum)
		out.AddInt(player.Type)
		out.AddInt(player.Race)
		fixed := 0
		if player.StartingPos.Fixed {
			fixed = 1
		}
		out.AddInt(fixed)
		out.AddString(player.Name)
		out.AddFloat(player.StartingPos.X)
		out.AddFloat(player.StartingPos.Y)
		out.AddInt(int(player.AllyLowPriorities))
		out.AddInt(int(player.AllyHighPriorities))
		out.AddInt(int(player.EnemyLowPriorities))
		out.AddInt(int(player.EnemyHighPriorities))
	}

	out.AddInt(len(infoJson.Forces))
	for _, force := range infoJson.Forces {
		out.AddInt(forceFlagsToInt(force.Flags))
		out.AddInt(int(force.Players))
		out.AddString(force.Name)
	}

	out.AddInt(len(infoJson.Upgrades))
	for _, upgrade := range infoJson.Upgrades {
		out.AddInt(int(upgrade.PlayerFlags))
		out.AddChars(upgrade.UpgradeID)
		out.AddInt(int(upgrade.Level))
		out.AddInt(int(upgrade.Availability))
	}

	out.AddInt(len(infoJson.TechBlacklist))
	for _, tech := range infoJson.TechBlacklist {
		out.AddInt(int(tech.PlayerFlags))
		out.AddChars(tech.TechID)
	}

	out.AddInt(len(infoJson.RandomUnitTables))
	for _, table := range infoJson.RandomUnitTables {
		out.AddInt(int(table.ID))
		out.AddString(table.Name)
		out.AddInt(len(table.Positions))
		for _, pos := range table.Positions {
			out.AddInt(int(pos))
		}
		out.AddInt(len(table.Chances))
		for _, chance := range table.Chances {
			out.AddInt(int(chance.Chance))
			for _, unitID := range chance.UnitIDs {
				out.AddChars(unitID)
			}
		}
	}

	out.AddInt(len(infoJson.RandomItemTables))
	for _, table := range infoJson.RandomItemTables {
		out.AddInt(int(table.ID))
		out.AddString(table.Name)
		out.AddInt(len(table.Rows))
		for _, row := range table.Rows {
			out.AddInt(len(row.Objects))
			for _, obj := range row.Objects {
				out.AddInt(int(obj.Chance))
				out.AddChars(obj.ObjectID)
			}
		}
	}

	return wc3.WarResult{Buffer: out.GetBuffer()}
}

func (*InfoTranslator) warToJSON(buffer []byte) wc3.JsonResult[data.Info] {
	result := defaultInfo()
	buf := wc3.NewW3Buffer(buffer)

	result.FileVersion = buf.ReadInt()
	result.Saves = buf.ReadInt()
	result.EditorVersion = buf.ReadInt()

	result.GameVersion = data.GameVersion{
		Major: int(buf.ReadInt()),
		Minor: int(buf.ReadInt()),
		Patch: int(buf.ReadInt()),
		Build: int(buf.ReadInt()),
	}

	result.Map.Name = buf.ReadString()
	result.Map.Author = buf.ReadString()
	result.Map.Description = buf.ReadString()
	result.Map.RecommendedPlayers = buf.ReadString()

	for i := 0; i < 8; i++ {
		result.Camera.Bounds[i] = buf.ReadFloat()
	}
	for i := 0; i < 4; i++ {
		result.Camera.Complements[i] = buf.ReadInt()
	}

	result.Map.PlayableArea = data.PlayableMapArea{
		Width:  int(buf.ReadInt()),
		Height: int(buf.ReadInt()),
	}

	flags := buf.ReadInt()
	result.Map.Flags = intToMapFlags(flags)
	result.Map.MainTileType = buf.ReadChars(1)

	result.LoadingScreen.Background = int(buf.ReadInt())
	result.LoadingScreen.Path = buf.ReadString()
	result.LoadingScreen.Text = buf.ReadString()
	result.LoadingScreen.Title = buf.ReadString()
	result.LoadingScreen.Subtitle = buf.ReadString()

	result.GameDataSet = buf.ReadInt()

	result.Prologue = data.Prologue{
		Path:     buf.ReadString(),
		Text:     buf.ReadString(),
		Title:    buf.ReadString(),
		Subtitle: buf.ReadString(),
	}

	result.Fog = data.Fog{
		Type:        int(buf.ReadInt()),
		StartHeight: buf.ReadFloat(),
		EndHeight:   buf.ReadFloat(),
		Density:     buf.ReadFloat(),
		Color: [4]byte{
			buf.ReadByte(), buf.ReadByte(), buf.ReadByte(), buf.ReadByte(),
		},
	}

	result.GlobalWeather = buf.ReadInt()
	result.CustomSoundEnv = buf.ReadString()
	result.CustomLightEnv = buf.ReadByte()
	result.Water = [4]byte{
		buf.ReadByte(), buf.ReadByte(), buf.ReadByte(), buf.ReadByte(),
	}

	result.ScriptLanguage = data.ScriptLanguage(buf.ReadInt())
	result.SupportedModes = data.SupportedModes(buf.ReadInt())
	result.GameDataVersion = buf.ReadInt()

	result.DefaultCameraZoom = buf.ReadInt()
	result.MaxCameraZoom = buf.ReadInt()
	result.MinCameraZoom = buf.ReadInt()

	numPlayers := int(buf.ReadInt())
	for i := 0; i < numPlayers; i++ {
		player := data.Player{}
		player.PlayerNum = int(buf.ReadInt())
		player.Type = int(buf.ReadInt())
		player.Race = int(buf.ReadInt())
		fixed := buf.ReadInt() == 1
		player.Name = buf.ReadString()
		player.StartingPos = data.PlayerStartingPosition{
			X:     buf.ReadFloat(),
			Y:     buf.ReadFloat(),
			Fixed: fixed,
		}
		player.AllyLowPriorities = buf.ReadInt()
		player.AllyHighPriorities = buf.ReadInt()
		player.EnemyLowPriorities = buf.ReadInt()
		player.EnemyHighPriorities = buf.ReadInt()
		result.Players = append(result.Players, player)
	}

	numForces := int(buf.ReadInt())
	for i := 0; i < numForces; i++ {
		forceFlag := buf.ReadInt()
		result.Forces = append(result.Forces, data.Force{
			Flags:   intToForceFlags(forceFlag),
			Players: buf.ReadInt(),
			Name:    buf.ReadString(),
		})
	}

	numUpgrades := int(buf.ReadInt())
	for i := 0; i < numUpgrades; i++ {
		result.Upgrades = append(result.Upgrades, data.UpgradeAvailable{
			PlayerFlags:  buf.ReadInt(),
			UpgradeID:    buf.ReadChars(4),
			Level:        buf.ReadInt(),
			Availability: buf.ReadInt(),
		})
	}

	numTech := int(buf.ReadInt())
	for i := 0; i < numTech; i++ {
		result.TechBlacklist = append(result.TechBlacklist, data.TechUnavailable{
			PlayerFlags: buf.ReadInt(),
			TechID:      buf.ReadChars(4),
		})
	}

	numUnitTable := int(buf.ReadInt())
	for i := 0; i < numUnitTable; i++ {
		table := data.RandomUnitTable{
			ID:   buf.ReadInt(),
			Name: buf.ReadString(),
		}
		numPositions := int(buf.ReadInt())
		for j := 0; j < numPositions; j++ {
			table.Positions = append(table.Positions, buf.ReadInt())
		}
		numChances := int(buf.ReadInt())
		for j := 0; j < numChances; j++ {
			chance := data.RandomUnitChance{Chance: buf.ReadInt()}
			for k := 0; k < numPositions; k++ {
				chance.UnitIDs = append(chance.UnitIDs, buf.ReadChars(4))
			}
			table.Chances = append(table.Chances, chance)
		}
		result.RandomUnitTables = append(result.RandomUnitTables, table)
	}

	numItemTable := int(buf.ReadInt())
	for i := 0; i < numItemTable; i++ {
		table := data.RandomItemTable{
			ID:   buf.ReadInt(),
			Name: buf.ReadString(),
		}
		itemSetsCurrentTable := int(buf.ReadInt())
		for j := 0; j < itemSetsCurrentTable; j++ {
			row := data.ObjectPool{Type: 2}
			itemsInItemSet := int(buf.ReadInt())
			for k := 0; k < itemsInItemSet; k++ {
				row.Objects = append(row.Objects, data.RandomObject{
					Chance:   buf.ReadInt(),
					ObjectID: buf.ReadChars(4),
				})
			}
			table.Rows = append(table.Rows, row)
		}
		result.RandomItemTables = append(result.RandomItemTables, table)
	}

	return wc3.JsonResult[data.Info]{JSON: result}
}

// DefaultInfo returns baseline war3map.w3i metadata for new maps.
func DefaultInfo() data.Info {
	return defaultInfo()
}

func defaultInfo() data.Info {
	return data.Info{
		FileVersion:   33,
		Saves:         1,
		EditorVersion: 6116,
		GameVersion: data.GameVersion{
			Major: 2,
			Minor: 0,
			Patch: 3,
			Build: 22978,
		},
		Map: data.MapInfo{
			PlayableArea: data.PlayableMapArea{Width: 64, Height: 64},
			Flags: data.MapFlags{
				IsMeleeMap: true,
				NonDefaultTilesetMapSizeLargeNeverBeenReducedToMedium: true,
				MaskedPartiallyVisible:                                true,
				MapPropertiesMenuOpenedAtLeastOnce:                    true,
				WaterWavesOnCliffShores:                               true,
				WaterWavesOnRollingShores:                             true,
				UseItemClassificationSystem:                           true,
			},
			MainTileType: "L",
		},
		LoadingScreen: data.LoadingScreen{Background: -1},
		Prologue:      data.Prologue{},
		Fog: data.Fog{
			StartHeight: 3000,
			EndHeight:   5000,
			Density:     0.5,
			Color:       [4]byte{0, 0, 0, 255},
		},
		Water:             [4]byte{255, 255, 255, 255},
		ScriptLanguage:    data.ScriptLanguageJASS,
		SupportedModes:    data.SupportedModesBoth,
		GameDataVersion:   1,
		DefaultCameraZoom: 1650,
		MaxCameraZoom:     1650,
		MinCameraZoom:     1650,
		Players: []data.Player{{
			PlayerNum: 0,
			Type:      1, // file format: 1=human
			Race:      1, // human
			Name:      "Player 1",
			StartingPos: data.PlayerStartingPosition{
				X: 0, Y: 0, Fixed: false,
			},
		}},
		Forces: []data.Force{{
			Players: -1,
			Name:    "",
		}},
		Upgrades:         []data.UpgradeAvailable{},
		TechBlacklist:    []data.TechUnavailable{},
		RandomUnitTables: []data.RandomUnitTable{},
		RandomItemTables: []data.RandomItemTable{},
	}
}

func mapFlagsToInt(f data.MapFlags) int {
	flags := 0
	if f.HideMinimapInPreview {
		flags |= 0x0001
	}
	if f.ModifyAllyPriorities {
		flags |= 0x0002
	}
	if f.IsMeleeMap {
		flags |= 0x0004
	}
	if f.NonDefaultTilesetMapSizeLargeNeverBeenReducedToMedium {
		flags |= 0x0008
	}
	if f.MaskedPartiallyVisible {
		flags |= 0x0010
	}
	if f.FixedPlayerSetting {
		flags |= 0x0020
	}
	if f.UseCustomForces {
		flags |= 0x0040
	}
	if f.UseCustomTechtree {
		flags |= 0x0080
	}
	if f.UseCustomAbilities {
		flags |= 0x0100
	}
	if f.UseCustomUpgrades {
		flags |= 0x0200
	}
	if f.MapPropertiesMenuOpenedAtLeastOnce {
		flags |= 0x0400
	}
	if f.WaterWavesOnCliffShores {
		flags |= 0x0800
	}
	if f.WaterWavesOnRollingShores {
		flags |= 0x1000
	}
	if f.UseTerrainFog {
		flags |= 0x2000
	}
	if f.TftRequired {
		flags |= 0x4000
	}
	if f.UseItemClassificationSystem {
		flags |= 0x8000
	}
	if f.EnableWaterTinting {
		flags |= 0x10000
	}
	if f.UseAccurateProbabilityForCalculations {
		flags |= 0x20000
	}
	if f.UseCustomAbilitySkins {
		flags |= 0x40000
	}
	return flags
}

func intToMapFlags(flags int32) data.MapFlags {
	return data.MapFlags{
		HideMinimapInPreview: flags&0x0001 != 0,
		ModifyAllyPriorities: flags&0x0002 != 0,
		IsMeleeMap:           flags&0x0004 != 0,
		NonDefaultTilesetMapSizeLargeNeverBeenReducedToMedium: flags&0x0008 != 0,
		MaskedPartiallyVisible:                                flags&0x0010 != 0,
		FixedPlayerSetting:                                    flags&0x0020 != 0,
		UseCustomForces:                                       flags&0x0040 != 0,
		UseCustomTechtree:                                     flags&0x0080 != 0,
		UseCustomAbilities:                                    flags&0x0100 != 0,
		UseCustomUpgrades:                                     flags&0x0200 != 0,
		MapPropertiesMenuOpenedAtLeastOnce:                    flags&0x0400 != 0,
		WaterWavesOnCliffShores:                               flags&0x0800 != 0,
		WaterWavesOnRollingShores:                             flags&0x1000 != 0,
		UseTerrainFog:                                         flags&0x2000 != 0,
		TftRequired:                                           flags&0x4000 != 0,
		UseItemClassificationSystem:                           flags&0x8000 != 0,
		EnableWaterTinting:                                    flags&0x10000 != 0,
		UseAccurateProbabilityForCalculations:                 flags&0x20000 != 0,
		UseCustomAbilitySkins:                                 flags&0x40000 != 0,
	}
}

func forceFlagsToInt(f data.ForceFlags) int {
	flags := 0
	if f.Allied {
		flags |= 0x0001
	}
	if f.AlliedVictory {
		flags |= 0x0002
	}
	if f.ShareVision {
		flags |= 0x0008
	}
	if f.ShareUnitControl {
		flags |= 0x0010
	}
	if f.ShareAdvUnitControl {
		flags |= 0x0020
	}
	return flags
}

func intToForceFlags(forceFlag int32) data.ForceFlags {
	return data.ForceFlags{
		Allied:              forceFlag&0b1 != 0,
		AlliedVictory:       forceFlag&0b10 != 0,
		ShareVision:         forceFlag&0b1000 != 0,
		ShareUnitControl:    forceFlag&0b10000 != 0,
		ShareAdvUnitControl: forceFlag&0b100000 != 0,
	}
}
