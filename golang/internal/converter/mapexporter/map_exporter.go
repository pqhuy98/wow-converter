package mapexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	creature "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/creature"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	"github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/wc3/translators"
	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

// MapExportConcurrency caps parallel workers for map export tasks (ADT tiles, creatures).
const MapExportConcurrency = 8

// MapExportWorkerCount returns bounded worker count for map export tasks.
func MapExportWorkerCount() int {
	n := config.MaxConcurrency()
	if n > MapExportConcurrency {
		return MapExportConcurrency
	}
	return n
}

// MapExporter orchestrates WoW -> WC3 map conversion.
type MapExporter struct {
	Config           config.Config
	MapExportConfig  *MapExportConfig
	MapManager       *extra.MapManager
	WowObjectManager *common.WowObjectManager
	wowClient        client.Client
	tileRegistry     *common.TileRegistry
	filterDoodads    func(id string, typ common.WowObjectType) bool
	preloadLocations [][2]float32
	water            *mapterrain.MapWater
	navigation       *mapterrain.TerrainNavigation
	terrainConverter *mapterrain.Wc3Converter
	factions         *creature.FactionPlan
	deathSounds      map[string]string
	warnings         []string
	terrainAssets    map[string]bool
}

// NewMapExporter creates an exporter. mapCfg must be non-nil and is mutated in place during export.
func NewMapExporter(cfg config.Config, mapCfg *MapExportConfig, wowClient client.Client, tileRegistry *common.TileRegistry) *MapExporter {
	if mapCfg == nil {
		panic("mapexporter: nil MapExportConfig")
	}
	return &MapExporter{
		Config:          cfg,
		MapExportConfig: mapCfg,
		MapManager:      extra.NewMapManager(),
		wowClient:       wowClient,
		tileRegistry:    tileRegistry,
	}
}

// ParseObjects reads terrains, doodads, and creatures.
func (e *MapExporter) ParseObjects(ctx context.Context, filter func(id string, typ common.WowObjectType) bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mc := e.MapExportConfig
	e.WowObjectManager = common.NewWowObjectManager(e.Config, e.wowClient, e.tileRegistry)
	e.filterDoodads = func(id string, typ common.WowObjectType) bool {
		enabled := mc.Doodads.Enable.Others
		switch typ {
		case common.WowObjectADT:
			enabled = mc.Doodads.Enable.ADT
		case common.WowObjectWMO:
			enabled = mc.Doodads.Enable.WMO
		case common.WowObjectM2:
			enabled = mc.Doodads.Enable.M2
		case common.WowObjectGobj:
			enabled = mc.Doodads.Enable.Gobj
		case common.WowObjectUnit:
			enabled = mc.Doodads.Enable.Unit
		}
		return enabled && typ != common.WowObjectUnit && (filter == nil || filter(id, typ))
	}

	if len(mc.WMOSet) > 0 {
		if err := e.WowObjectManager.ReadTerrainsDoodads(ctx, mc.WMOSet, e.filterDoodads); err != nil {
			return err
		}
	}
	paths := buildPaths("**/"+mc.WowExportFolder, mc.Min, mc.Max)
	if err := e.WowObjectManager.ReadTerrainsDoodads(ctx, paths, e.filterDoodads); err != nil {
		return err
	}
	if err := e.WowObjectManager.ReadCreatures(ctx, mc.MapID); err != nil {
		return err
	}
	if mc.Doodads.Enable.Gobj {
		if err := e.WowObjectManager.ReadGameObjects(ctx, mc.MapID); err != nil {
			return err
		}
	}

	log.Printf("Total objects: %d", len(e.WowObjectManager.Objects))
	typeCount := map[string]int{}
	for _, obj := range e.WowObjectManager.Objects {
		typeCount[string(obj.Type)]++
	}
	log.Printf("Object type count: %v", typeCount)
	log.Printf("Rotating roots at center by %.0f degrees", mc.MapAngleDeg)
	e.WowObjectManager.RotateRootsAtCenter([3]float64{0, 0, math.Radians(-90 + mc.MapAngleDeg)})
	if e.tileRegistry != nil {
		e.tileRegistry.TrimAfterParse()
	}
	return nil
}

// ExportTerrainsDoodads generates terrain and exports doodad assets.
func (e *MapExporter) ExportTerrainsDoodads(ctx context.Context, outputDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	wc3 := mapterrain.NewWc3Converter(*e.MapExportConfig)
	e.terrainConverter = wc3
	terrain, err := wc3.GenerateTerrainWithHeight(e.WowObjectManager)
	if err != nil {
		return err
	}
	if e.MapExportConfig.TerrainAsWc3Tilesets {
		if err := wc3.ApplyWc3Tileset(ctx, &terrain, e.WowObjectManager, e.wowClient, e.Config); err != nil {
			return err
		}
	}
	water, err := mapterrain.CollectMapWater(ctx, e.WowObjectManager, terrain, e.MapExportConfig)
	if err != nil {
		return err
	}
	e.water = water
	navigation, err := mapterrain.GenerateReachableTerrain(ctx, e.WowObjectManager, terrain, e.MapExportConfig, e.Config.RawModelScaleUp, water)
	if err != nil {
		return err
	}
	e.navigation = navigation
	if err := water.Project(ctx, terrain); err != nil {
		return err
	}
	if wc3.Tileset != nil {
		if err := wc3.FinishTilesetModels(ctx, e.WowObjectManager, terrain, wc3.Tileset, e.Config); err != nil {
			return err
		}
		wc3.BlockModelCells(navigation)
	}
	if err := wc3.PrepareAdtChunks(ctx, e.WowObjectManager, e.Config.AssetPrefix); err != nil {
		return err
	}
	e.MapManager.SetTerrain(terrain)
	if navigation != nil {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}
	} else {
		// Do not retain a previous generation's pathing when navigation is disabled
		// or no grounded NPCs could seed this export.
		for _, name := range []string{"war3map.wpm", "terrain-navigation.json", "terrain-navigation.png"} {
			if err := os.Remove(filepath.Join(outputDir, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}

	tiltedDoodads, _ := wc3.PlaceDoodads(e.MapManager, e.WowObjectManager, func(obj *common.WowObject) bool {
		return e.filterDoodads(obj.ID, obj.Type)
	})
	e.preloadLocations = wc3.PreloadLocations
	if navigation != nil {
		if e.MapExportConfig.PlacePathBlockers {
			if err := navigation.PlaceBlockers(e.MapManager, outputDir); err != nil {
				return err
			}
		} else if err := navigation.Write(outputDir, false); err != nil {
			return err
		}
	}
	log.Printf("Total doodads: %d", len(e.MapManager.Doodads))
	log.Printf("Total doodad types: %d", len(e.MapManager.DoodadTypes))
	log.Printf("Doodad instances with custom pitch/roll: %d", tiltedDoodads)

	if len(e.MapManager.Doodads) > 130000 {
		return fmt.Errorf("too many doodads: %d, limit is 130000", len(e.MapManager.Doodads))
	}

	am := e.WowObjectManager.AssetManager
	usedModelPaths := map[string]struct{}{}
	modelStem := func(path string) string {
		path = strings.ReplaceAll(path, "\\", "/")
		if ext := strings.ToLower(filepath.Ext(path)); ext == ".mdx" || ext == ".mdl" {
			path = path[:len(path)-len(ext)]
		}
		return path
	}
	collectModelPath := func(mods []data.Modification) {
		for _, m := range mods {
			if (m.ID == "dfil" || m.ID == "bfil") && m.Type == data.ModificationString {
				if s, ok := m.Value.(string); ok {
					usedModelPaths[modelStem(s)] = struct{}{}
				}
			}
		}
	}
	for _, t := range e.MapManager.DoodadTypes {
		collectModelPath(t.Data)
	}
	for _, t := range e.MapManager.DestructibleTypes {
		collectModelPath(t.Data)
	}
	for k, model := range am.Models() {
		rel := modelStem(model.MDL.Model.Name)
		if _, ok := usedModelPaths[rel]; !ok {
			delete(am.Models(), k)
		}
	}

	var usedTextures []string
	for _, model := range am.Models() {
		for _, tex := range model.MDL.Textures {
			var p string
			if tex.WowData.PngPath != "" {
				p = strings.ReplaceAll(tex.WowData.PngPath, "\\", "/")
			} else if tex.Image != "" {
				p = strings.ReplaceAll(strings.ReplaceAll(tex.Image, ".blp", ".png"), "\\", "/")
			}
			if p != "" {
				usedTextures = append(usedTextures, p)
				limit := e.MapExportConfig.Doodads.MaxTextureSize
				if model.TightBounds {
					limit = 0 // ADT chunks retain the terrain bake resolution.
				}
				am.SetTextureMaxSize(p, limit)
			}
		}
	}
	am.PurgeTextures(usedTextures)

	if _, err := am.ExportTextures(ctx, outputDir); err != nil {
		return err
	}
	if err := e.prepareTerrainAssets(ctx, outputDir); err != nil {
		return err
	}
	if err := am.ExportModels(ctx, outputDir); err != nil {
		return err
	}
	manifest, err := json.MarshalIndent(wc3.GameObjects, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "gameobjects.json"), manifest, 0o644); err != nil {
		return err
	}
	am.ReleaseAfterExport()
	return nil
}

// ExportCreatures places units and exports creature models.
func (e *MapExporter) ExportCreatures(ctx context.Context, outputDir string, onProgress func(completed, total int)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	mc := e.MapExportConfig
	e.deathSounds = nil
	e.warnings = nil
	if !mc.Creatures.Enable {
		if err := os.Remove(filepath.Join(outputDir, "units.json")); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	var voices creature.CreatureVoices
	if !mc.Creatures.AllAreDoodads {
		var err error
		voices, err = creature.LoadCreatureVoices()
		if err != nil {
			return err
		}
		if err := creature.PrepareCreatureSelection(ctx, e.wowClient, e.WowObjectManager); err != nil {
			return err
		}
	}
	units := creature.PlaceUnits(*mc, e.MapManager, e.WowObjectManager)
	if !mc.Creatures.AllAreDoodads {
		if plan, err := creature.ConvertFactionPlayers(e.MapExportConfig.PlayerFaction, e.MapManager, units); err != nil {
			return err
		} else {
			e.factions = plan
		}
		creature.ApplyCreatureVoices(e.MapManager, units, voices)
		if sounds, err := creature.ExportCreatureDeathSounds(ctx, e.wowClient, outputDir, units); err != nil {
			return err
		} else {
			e.deathSounds = sounds
		}
	}
	log.Printf("Created %d custom unit types", len(e.MapManager.UnitTypes))
	log.Printf("Placed %d unit instances", len(e.MapManager.Units))

	creatures := make([]azerothcore.Creature, 0, len(units))
	for _, u := range units {
		creatures = append(creatures, u.Creature)
	}
	start := time.Now()
	creatureConfig := e.Config
	creatureConfig.MaxTextureSize = mc.Creatures.MaxTextureSize
	exported, err := creature.ExportCreatureAssets(ctx, creatures, outputDir, creatureConfig, e.wowClient, MapExportWorkerCount(), mc.Creatures.AllAreDoodads, onProgress)
	if err != nil {
		return err
	}
	assets, icons := exported.Models, exported.Icons
	if exported.IconError != nil {
		log.Printf("NPC icons unavailable: %v (using Footman icons)", exported.IconError)
		models := map[string]bool{}
		failed := 0
		for _, asset := range assets {
			if !models[asset.ModelPath] {
				models[asset.ModelPath] = true
				if icons[asset.ModelPath] == "" {
					failed++
				}
			}
		}
		if failed > 0 {
			e.warnings = append(e.warnings, fmt.Sprintf("NPC icon rendering failed for %d/%d models.", failed, len(models)))
		}
	}
	creature.ApplyCreatureAssets(e.MapManager, units, assets, icons)
	if err := creature.EnrichCreatureModelFields(ctx, outputDir, e.MapManager, units, assets); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	report, err := json.MarshalIndent(struct {
		Units []creature.PlacedUnit `json:"units"`
	}{units}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "units.json"), report, 0644); err != nil {
		return err
	}
	log.Printf("Exported all unit assets in %s", ansi.Yellowf("%.2fs", time.Since(start).Seconds()))
	log.Printf("Done")
	return nil
}

// SaveWar3mapFiles copies template and writes map binaries.
func (e *MapExporter) SaveWar3mapFiles(ctx context.Context, outputDir, mapName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := mapterrain.RemoveLegacyWaterMeshes(outputDir); err != nil {
		return err
	}
	templateDir, err := workspace.ResolveTemplateEmptyDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		if err := copyDir(ctx, templateDir, outputDir); err != nil {
			return err
		}
	} else {
		entries, err := os.ReadDir(templateDir)
		if err != nil {
			return err
		}
		for _, ent := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			dst := filepath.Join(outputDir, ent.Name())
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				if err := copyDir(ctx, filepath.Join(templateDir, ent.Name()), dst); err != nil {
					return err
				}
			}
		}
	}
	e.MapManager.EnsureMapInfo(mapName)
	if err := e.inheritTemplateMapSettings(templateDir); err != nil {
		return err
	}
	if !e.MapExportConfig.Creatures.Enable || e.MapExportConfig.Creatures.AllAreDoodads {
		e.deathSounds = nil
		e.factions = &creature.FactionPlan{
			UserSlot:  0,
			Players:   []creature.FactionPlayer{{Slot: 0, Name: "Player 1", FactionName: "Player 1"}},
			Reactions: [][]creature.FactionReaction{{creature.FactionFriendly}},
		}
	} else if e.factions == nil {
		var err error
		e.factions, err = creature.MakeFactionPlan(e.MapExportConfig.PlayerFaction, nil, nil)
		if err != nil {
			return err
		}
	}
	creature.ConfigureFactionSlots(e.MapManager, e.factions)
	ensureFactionStartLocations(e.MapManager, e.factions.UserSlot)
	e.MapManager.Info.ScriptLanguage = data.ScriptLanguageLua
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.MapManager.Save(outputDir); err != nil {
		return err
	}
	factionsJSON, err := json.MarshalIndent(e.factions, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDir, "factions.json"), factionsJSON, 0644); err != nil {
		return err
	}
	if err := e.exportMinimap(ctx, outputDir); err != nil {
		return err
	}
	if err := e.saveStartupScripts(ctx, templateDir, outputDir); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(outputDir, "war3map.shd"))
	// The empty-map template contains a WPM too; do not restore it after an
	// opt-out export removed cached navigation pathing.
	if !e.MapExportConfig.PlacePathBlockers {
		if err := os.Remove(filepath.Join(outputDir, "war3map.wpm")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := e.terrainConverter.WriteWc3Tileset(outputDir); err != nil {
		return err
	}
	if e.water != nil && e.water.HasCells() {
		if err := e.water.Write(ctx, e.MapManager.GetTerrain(), e.navigation, outputDir, e.MapExportConfig.PlacePathBlockers); err != nil {
			return err
		}
	} else {
		for _, name := range []string{"terrain-water.json", "terrain-water.png"} {
			if err := os.Remove(filepath.Join(outputDir, name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return e.cleanTerrainAssets(ctx, outputDir)
}

func (e *MapExporter) inheritTemplateMapSettings(templateDir string) error {
	raw, err := os.ReadFile(filepath.Join(templateDir, "war3map.w3i"))
	if err != nil {
		return err
	}
	template := translators.InfoTranslator{}.WarToJSON(raw).JSON
	info := &e.MapManager.Info
	// Alpha-tile color is only serialized in v39+. Seed the new format's
	// companion settings from the template when upgrading older map metadata.
	if info.FileVersion < template.FileVersion {
		info.FileVersion = template.FileVersion
		info.EditorVersion = template.EditorVersion
		info.GameVersion = template.GameVersion
		info.RaceHud = template.RaceHud
		info.Fog3 = template.Fog3
		info.Water3 = template.Water3
	}
	info.Water3.AlphaTileMinimapColor = template.Water3.AlphaTileMinimapColor
	info.Map.Flags.UseAlphaTileMinimapColor = template.Map.Flags.UseAlphaTileMinimapColor
	info.LoadingScreen = template.LoadingScreen
	// Inline template strings so cached output cannot resolve them through an
	// older WTS table (or require replacing unrelated map strings).
	var wts []byte
	for _, text := range []*string{&info.LoadingScreen.Text, &info.LoadingScreen.Title, &info.LoadingScreen.Subtitle} {
		if !strings.HasPrefix(*text, "TRIGSTR_") {
			continue
		}
		id, err := strconv.Atoi(strings.TrimPrefix(*text, "TRIGSTR_"))
		if err != nil {
			return fmt.Errorf("invalid template loading screen string %q: %w", *text, err)
		}
		if wts == nil {
			wts, err = os.ReadFile(filepath.Join(templateDir, "war3map.wts"))
			if err != nil {
				return err
			}
		}
		pattern := fmt.Sprintf(`(?ms)^STRING[ \t]+0*%d[ \t]*\r?\n(?:[^\n]*\n)?\{\r?\n(.*?)\r?\n\}`, id)
		match := regexp.MustCompile(pattern).FindSubmatch(wts)
		if match == nil {
			return fmt.Errorf("template loading screen string %q missing from war3map.wts", *text)
		}
		*text = string(match[1])
	}
	return nil
}

func buildPaths(prefix string, min, max math.Vector2) []string {
	var res []string
	for i := int(min[0]); i <= int(max[0]); i++ {
		for j := int(min[1]); j <= int(max[1]); j++ {
			res = append(res, filepath.Join(prefix, fmt.Sprintf("adt_%d_%d.obj", i, j)))
		}
	}
	return res
}

func copyDir(ctx context.Context, src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(ctx, path, target)
	})
}

func copyFile(ctx context.Context, src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
