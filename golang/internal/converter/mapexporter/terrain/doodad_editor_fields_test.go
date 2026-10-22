package terrain

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	settings "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestDoodadEditorFieldsFollowSourceHierarchy(t *testing.T) {
	node := func(typ common.WowObjectType, name string, children ...*common.WowObject) *common.WowObject {
		model := mdl.New(mdl.NewMDLOptions{Name: name})
		model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{
			{Position: [3]float64{0, 0, 0}}, {Position: [3]float64{100, 100, 100}},
		}}}
		return &common.WowObject{Type: typ, ScaleFactor: 1, Model: &common.Model{RelativePath: name, MDL: model}, Children: children}
	}
	for _, tc := range []struct {
		name     string
		roots    []*common.WowObject
		expected []string // One field per placement in source traversal order.
	}{
		{
			name: "ADT children and excluded nested props, units, gameobjects",
			roots: []*common.WowObject{
				node(common.WowObjectADT, "terrain.mdx",
					node(common.WowObjectWMO, "building.mdx", node(common.WowObjectM2, "shared.mdx")),
					node(common.WowObjectM2, "shared.mdx"),
					node(common.WowObjectUnit, "shared.mdx"),
					node(common.WowObjectGobj, "shared.mdx")),
				node(common.WowObjectADT, "terrain2.mdx"),
				node(common.WowObjectWMO, "building.mdx"),
			},
			expected: []string{"dimc", "doct", "", "doct", "", "", "dimc", ""},
		},
		{
			name: "WMO root fallback only",
			roots: []*common.WowObject{
				node(common.WowObjectWMO, "building.mdx",
					node(common.WowObjectWMO, "building.mdx", node(common.WowObjectM2, "shared.mdx"))),
				node(common.WowObjectM2, "shared.mdx"),
			},
			expected: []string{"dimc", "", "", ""},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
			manager.Roots = tc.roots
			mm := extra.NewMapManager()
			mm.SetTerrain(GetInitialTerrain(2, 2))
			_, err := NewWc3Converter(settings.DefaultMapExportConfig()).PlaceDoodads(mm, manager, func(*common.WowObject) bool { return true })
			if err != nil {
				t.Fatal(err)
			}
			mm.EnsureMapInfo("editor-fields")
			dir := t.TempDir()
			if err := mm.Save(dir); err != nil {
				t.Fatal(err)
			}
			saved := extra.NewMapManager()
			if err := saved.Load(dir); err != nil {
				t.Fatal(err)
			}
			if len(saved.Doodads) != len(tc.expected) {
				t.Fatalf("got %d placements, want %d", len(saved.Doodads), len(tc.expected))
			}
			types := map[string][]data.Modification{}
			for _, typ := range append(saved.DoodadTypes, saved.DestructibleTypes...) {
				types[typ.Code] = typ.Data
			}
			for i, placed := range saved.Doodads {
				got := ""
				for _, field := range types[placed.Doodad.Type] {
					if field.ID != "dimc" && field.ID != "doct" {
						continue
					}
					if got != "" {
						t.Fatalf("placement %d has both fields", i)
					}
					got = field.ID
					if field.ID == "doct" && (field.Type != data.ModificationString || field.Value != "Auto") {
						t.Fatalf("doct must serialize as the string Auto: %+v", field)
					}
					if field.ID == "dimc" && (field.Type != data.ModificationInt || field.Value != int32(1)) {
						t.Fatalf("dimc must serialize as integer 1: %+v", field)
					}
				}
				if got != tc.expected[i] {
					t.Errorf("placement %d field %q, want %q", i, got, tc.expected[i])
				}
			}
		})
	}
}
