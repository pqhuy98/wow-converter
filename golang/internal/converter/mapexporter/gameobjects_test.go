package mapexporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

// Run against a real generated map with WOW_GAMEOBJECT_TEST_MAP set to its directory.
func TestExportedGameObjectPlacements(t *testing.T) {
	dir := os.Getenv("WOW_GAMEOBJECT_TEST_MAP")
	if dir == "" {
		t.Skip("set WOW_GAMEOBJECT_TEST_MAP to verify a generated map")
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "gameobjects.json"))
	if err != nil {
		t.Fatal(err)
	}
	var objects []gameObjectPlacement
	if err := json.Unmarshal(manifest, &objects); err != nil {
		t.Fatal(err)
	}
	if len(objects) == 0 {
		t.Fatal("no gameobjects exported")
	}
	m := extra.NewMapManager()
	if err := m.Load(dir); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	usedPlacements := make(map[int]bool)
	for _, obj := range objects {
		if seen[obj.ID] {
			t.Fatalf("duplicate spawn %s", obj.ID)
		}
		seen[obj.ID] = true
		model := strings.ReplaceAll(obj.Model, "\\", "/")
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(model))); err != nil {
			t.Fatal(err)
		}
		matched := false
		isDestructible := false
		for _, typ := range m.DestructibleTypes {
			if typ.Code == obj.TypeCode {
				for _, field := range typ.Data {
					if field.ID == "bfil" && field.Value != obj.Model {
						t.Fatalf("%s definition model differs from manifest", obj.ID)
					}
				}
				isDestructible = true
				break
			}
		}
		if !isDestructible {
			t.Fatalf("%s has no destructible definition", obj.ID)
		}
		for i, placed := range m.Doodads {
			if usedPlacements[i] || placed.Doodad.Type != obj.TypeCode || placed.Position != obj.Position || placed.Scale != obj.Scale {
				continue
			}
			usedPlacements[i] = true
			matched = true
			break
		}
		if !matched {
			t.Fatalf("%s has no unused matching serialized placement", obj.ID)
		}
	}
	t.Logf("verified %d gameobject destructibles and model files", len(objects))
}

func TestGameObjectsSerializeAsDestructiblesWithIndependentPlacements(t *testing.T) {
	cfg := config.DefaultConfig()
	manager := common.NewWowObjectManager(cfg, nil, nil)
	model := mdl.New(mdl.NewMDLOptions{Name: "terrain.mdx"})
	model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{{Position: [3]float64{0, 0, 0}}, {Position: [3]float64{100, 100, 100}}}}}
	geometry := &common.Model{RelativePath: "shared.mdx", MDL: model}
	root := &common.WowObject{ID: "tile", Type: common.WowObjectADT, Model: geometry, ScaleFactor: 1}
	// A decorative object and two overlapping gameobjects share geometry.
	for i, typ := range []common.WowObjectType{common.WowObjectM2, common.WowObjectGobj, common.WowObjectGobj} {
		root.Children = append(root.Children, &common.WowObject{Type: typ, Model: geometry, Position: [3]float64{50, 50, 50}, ScaleFactor: float64(i + 1)})
	}
	manager.Roots = []*common.WowObject{root}
	mm := extra.NewMapManager()
	mm.SetTerrain(getInitialTerrain(2, 2))
	wc := NewWc3Converter(DefaultMapExportConfig())
	wc.config.Terrain.ClampPercent.Lower = 0
	wc.config.Terrain.ClampPercent.Upper = 1
	_, err := wc.PlaceDoodads(mm, manager, func(obj *common.WowObject) bool { return obj.Type != common.WowObjectADT })
	if err != nil {
		t.Fatal(err)
	}
	if len(mm.DestructibleTypes) != 1 || len(mm.DoodadTypes) != 1 || len(mm.Doodads) != 3 {
		t.Fatalf("wrong types/placements: %d %d %d", len(mm.DestructibleTypes), len(mm.DoodadTypes), len(mm.Doodads))
	}
	for _, placement := range mm.Doodads[1:] {
		typ, ok := placement.Type.(*extra.DoodadType)
		if !ok || !typ.IsDestructible {
			t.Fatal("gameobject became doodad")
		}
	}
	if mm.Doodads[2].Scale[0]/mm.Doodads[1].Scale[0] != 1.5 {
		t.Fatal("placement scale lost")
	}
	for _, field := range mm.DestructibleTypes[0].Data {
		if field.ID[0] == 'd' {
			t.Fatalf("doodad field in destructible: %s", field.ID)
		}
	}
	dir := t.TempDir()
	if err := mm.Save(dir); err != nil {
		t.Fatal(err)
	}
	loaded := extra.NewMapManager()
	if err := loaded.Load(dir); err != nil {
		t.Fatal(err)
	}
	if len(loaded.DestructibleTypes) != 1 || len(loaded.Doodads) != 3 {
		t.Fatal("destructibles lost during serialization")
	}
}
