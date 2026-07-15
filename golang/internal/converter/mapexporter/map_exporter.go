package mapexporter

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
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

// TerrainClampPercent configures Z clamp for terrain generation.
type TerrainClampPercent struct {
	Lower float64
	Upper float64
}

// DoodadEnable toggles doodad types during export.
type DoodadEnable struct {
	ADT    bool
	WMO    bool
	M2     bool
	Gobj   bool
	Unit   bool
	Others bool
}

// MapExportConfig drives a single map export run.
type MapExportConfig struct {
	MapID           int
	WowExportFolder string
	WMOSet          []string
	Min             math.Vector2
	Max             math.Vector2
	MapAngleDeg     float64
	Terrain         struct {
		ClampPercent TerrainClampPercent
	}
	Doodads struct {
		Enable                    DoodadEnable
		PitchRollThresholdRadians float64
	}
	Creatures struct {
		Enable        bool
		AllAreDoodads bool
	}
	UnitScale float64
}

// DefaultMapExportConfig returns defaults excluding map-specific fields.
func DefaultMapExportConfig() MapExportConfig {
	cfg := MapExportConfig{
		Terrain: struct{ ClampPercent TerrainClampPercent }{
			ClampPercent: TerrainClampPercent{Lower: 0, Upper: 1},
		},
		UnitScale: 1,
	}
	cfg.Doodads.Enable = DoodadEnable{ADT: true, WMO: true, M2: true, Gobj: true, Unit: true, Others: true}
	cfg.Doodads.PitchRollThresholdRadians = math.Radians(5)
	cfg.Creatures.Enable = true
	return cfg
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
func (e *MapExporter) ParseObjects(filter func(id string, typ common.WowObjectType) bool) error {
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
		if err := e.WowObjectManager.ReadTerrainsDoodads(mc.WMOSet, e.filterDoodads); err != nil {
			return err
		}
	}
	paths := buildPaths("**/"+mc.WowExportFolder, mc.Min, mc.Max)
	if err := e.WowObjectManager.ReadTerrainsDoodads(paths, e.filterDoodads); err != nil {
		return err
	}
	if err := e.WowObjectManager.ReadCreatures(mc.MapID); err != nil {
		return err
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
func (e *MapExporter) ExportTerrainsDoodads(outputDir string) error {
	wc3 := NewWc3Converter(*e.MapExportConfig)
	e.MapManager.SetTerrain(wc3.GenerateTerrainWithHeight(e.WowObjectManager))

	_, _ = wc3.PlaceDoodads(e.MapManager, e.WowObjectManager, func(obj *common.WowObject) bool {
		return e.filterDoodads(obj.ID, obj.Type)
	})
	log.Printf("Total doodads: %d", len(e.MapManager.Doodads))
	log.Printf("Total doodad types: %d", len(e.MapManager.DoodadTypes))
	pitchRollCount := 0
	for _, t := range e.MapManager.DoodadTypes {
		for _, m := range t.Data {
			if m.ID == "dprx" || m.ID == "dpro" {
				pitchRollCount++
				break
			}
		}
	}
	log.Printf("Doodad types with custom pitch/roll: %d", pitchRollCount)

	if len(e.MapManager.Doodads) > 130000 {
		return fmt.Errorf("too many doodads: %d, limit is 130000", len(e.MapManager.Doodads))
	}

	am := e.WowObjectManager.AssetManager
	usedModelPaths := map[string]struct{}{}
	collectModelPath := func(mods []data.Modification) {
		for _, m := range mods {
			if (m.ID == "dfil" || m.ID == "bfil") && m.Type == data.ModificationString {
				if s, ok := m.Value.(string); ok {
					usedModelPaths[strings.ReplaceAll(s, "\\", "/")] = struct{}{}
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
		rel := strings.ReplaceAll(model.MDL.Model.Name, "\\", "/")
		if _, ok := usedModelPaths[rel]; !ok {
			delete(am.Models(), k)
		}
	}

	var usedTextures []string
	for _, model := range am.Models() {
		for _, tex := range model.MDL.Textures {
			if tex.WowData.PngPath != "" {
				usedTextures = append(usedTextures, strings.ReplaceAll(tex.WowData.PngPath, "\\", "/"))
			} else if tex.Image != "" {
				usedTextures = append(usedTextures, strings.ReplaceAll(strings.ReplaceAll(tex.Image, ".blp", ".png"), "\\", "/"))
			}
		}
	}
	am.PurgeTextures(usedTextures)

	if _, err := am.ExportTextures(outputDir); err != nil {
		return err
	}
	if err := am.ExportModels(outputDir); err != nil {
		return err
	}
	am.ReleaseAfterExport()
	return nil
}

// ExportCreatures places units and exports creature models.
func (e *MapExporter) ExportCreatures(outputDir string, onProgress func(completed, total int)) error {
	mc := e.MapExportConfig
	if !mc.Creatures.Enable {
		return nil
	}
	wc3 := NewWc3Converter(*mc)
	units := wc3.PlaceUnits(e.MapManager, e.WowObjectManager)
	log.Printf("Created %d custom unit types", len(e.MapManager.UnitTypes))
	log.Printf("Placed %d unit instances", len(e.MapManager.Units))

	creatures := make([]azerothcore.Creature, 0, len(units))
	for _, u := range units {
		creatures = append(creatures, u.Creature)
	}
	start := time.Now()
	if err := character.ExportCreatureModels(creatures, outputDir, e.Config, e.wowClient, MapExportWorkerCount(), onProgress); err != nil {
		return err
	}
	log.Printf("Exported all unit assets in %s", ansi.Yellowf("%.2fs", time.Since(start).Seconds()))
	log.Printf("Done")
	return nil
}

// SaveWar3mapFiles copies template and writes map binaries.
func (e *MapExporter) SaveWar3mapFiles(outputDir, mapName string) error {
	templateDir, err := workspace.ResolveTemplateEmptyDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		if err := copyDir(templateDir, outputDir); err != nil {
			return err
		}
	} else {
		entries, _ := os.ReadDir(templateDir)
		for _, ent := range entries {
			dst := filepath.Join(outputDir, ent.Name())
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				if err := copyFile(filepath.Join(templateDir, ent.Name()), dst); err != nil {
					return err
				}
			}
		}
	}
	e.MapManager.EnsureMapInfo(mapName)
	if err := e.MapManager.Save(outputDir); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(outputDir, "war3map.shd"))
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

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
