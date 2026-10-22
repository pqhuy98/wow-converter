package terrain

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	settings "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestDoodadInstanceRotationPreservesSourceOrientation(t *testing.T) {
	rotations := []imath.EulerRotation{
		{}, {0, 0, 4.7}, {.01, -.02, 1.2}, {.4, -.7, 2.1}, {-2.1, 1.2, -.9},
		{math.Pi / 2, 0, .7}, {-math.Pi / 2, 0, -.3}, {math.Pi / 2, math.Pi, .8},
		{math.Pi/2 - 1e-6, 1e-6, 1.3}, {.8, math.Pi / 2, -.4},
	}
	for _, source := range rotations {
		yaw, roll, pitch := doodadInstanceRotation(source)
		for _, axis := range []imath.Vector3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} {
			want := imath.V3Rotate(axis, source)
			// Native doodads apply pitch, then roll, then yaw to a model vertex.
			got := imath.V3Rotate(axis, imath.Vector3{0, float64(pitch), 0})
			got = imath.V3Rotate(got, imath.Vector3{float64(roll), 0, 0})
			got = imath.V3Rotate(got, imath.Vector3{0, 0, imath.Radians(float64(yaw))})
			for i := range got {
				if math.Abs(got[i]-want[i]) > 2e-6 {
					t.Fatalf("rotation %v axis %v: got %v want %v", source, axis, got, want)
				}
			}
		}
	}
}

func TestDoodadTiltSharesTypesAndSurvivesMapSave(t *testing.T) {
	manager := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
	model := mdl.New(mdl.NewMDLOptions{Name: "shared.mdx"})
	model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{
		{Position: [3]float64{0, 0, 0}}, {Position: [3]float64{100, 100, 100}},
	}}}
	geometry := &common.Model{RelativePath: "shared.mdx", MDL: model}
	root := &common.WowObject{Type: common.WowObjectADT, ScaleFactor: 1, Model: geometry}
	rotations := []imath.EulerRotation{{}, {.01, -.02, 1.2}, {-.4, .7, 2.1}, {.6, -.8, -1.1}}
	for _, typ := range []common.WowObjectType{common.WowObjectM2, common.WowObjectGobj} {
		for i, rotation := range rotations {
			root.Children = append(root.Children, &common.WowObject{Type: typ, Model: geometry,
				Position: [3]float64{50, 50, 50}, Rotation: rotation, ScaleFactor: 1 + float64(i)*.1})
		}
	}
	manager.Roots = []*common.WowObject{root}
	mm := extra.NewMapManager()
	mm.SetTerrain(GetInitialTerrain(16, 16))
	converter := NewWc3Converter(settings.DefaultMapExportConfig())
	tilted, err := converter.PlaceDoodads(mm, manager, func(obj *common.WowObject) bool { return obj != root })
	if err != nil {
		t.Fatal(err)
	}
	if tilted != 6 || len(mm.Doodads) != 8 || len(mm.DoodadTypes) != 1 || len(mm.DestructibleTypes) != 1 {
		t.Fatalf("tilts created duplicate types: %d tilted, %d placements, %d doodad/%d destructible types", tilted, len(mm.Doodads), len(mm.DoodadTypes), len(mm.DestructibleTypes))
	}
	for _, typ := range []extra.DoodadType{mm.DoodadTypes[0], mm.DestructibleTypes[0]} {
		fields := fieldValues(t, typ.Data)
		prefix := "d"
		if typ.IsDestructible {
			prefix = "b"
		}
		if fields[prefix+"mar"] != float32(0) || fields[prefix+"map"] != float32(0) {
			t.Fatal("type tilt/terrain alignment was not cleared")
		}
	}
	for i, placement := range mm.Doodads {
		yaw, roll, pitch := doodadInstanceRotation(rotations[i%len(rotations)])
		if placement.Angle != yaw || placement.Roll != roll || placement.Pitch != pitch {
			t.Fatalf("placement %d lost tilt", i)
		}
	}
	dir := filepath.Join(t.TempDir(), "rotation.w3x")
	if err := mm.Save(dir); err != nil {
		t.Fatal(err)
	}
	loaded := extra.NewMapManager()
	if err := loaded.Load(dir); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Doodads) != len(mm.Doodads) || len(loaded.DoodadTypes) != 1 || len(loaded.DestructibleTypes) != 1 {
		t.Fatal("map save lost instances or recreated types")
	}
	for i, placement := range loaded.Doodads {
		before := mm.Doodads[i]
		if placement.Roll != before.Roll || placement.Pitch != before.Pitch || placement.Position != before.Position || placement.Scale != before.Scale {
			t.Fatalf("map save lost instance transform %d", i)
		}
	}
}
