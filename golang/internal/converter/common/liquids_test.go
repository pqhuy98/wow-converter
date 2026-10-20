package common

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/liquid"
)

func TestLiquidSoundBankIntegerWidths(t *testing.T) {
	// WDC's compressed fields retain their declared 8/16-bit Go types.
	for _, value := range []any{uint8(2), int8(2), uint16(2), int16(2), uint32(2), int32(2), int64(2), uint64(2)} {
		if gameObjectFileID(value) != 2 {
			t.Fatalf("magma SoundBank %T became ordinary water", value)
		}
	}
}

func TestADTWaterSurvivesRecenterRotationAndSnapshotRelease(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ExportAssetDir = t.TempDir()
	cfg.RawModelScaleUp = 2
	const path = "maps/adt_1_1"
	source := liquid.Surface{TypeID: 5, Cells: [][4]imath.Vector3{{{10, 20, 7}, {12, 20, 7}, {12, 22, 8}, {10, 22, 8}}}}
	snap := &exportadt.ConversionOutput{
		ObjectPath: path, ObjFilePath: filepath.Join(cfg.ExportAssetDir, path+".obj"),
		ObjText: "o adt\nv 10 20 0\nv 12 20 0\nv 12 22 1\nvt 0 0\nvt 1 0\nvt 1 1\nvn 0 0 1\nusemtl ground\nf 1/1/1 2/2/1 3/3/1\n",
		MtlText: "newmtl ground\nKd 1 1 1\n", Liquids: []liquid.Surface{source},
	}
	r := NewTileRegistry()
	r.Register(snap)
	m := NewWowObjectManager(cfg, nil, r)
	if err := m.ReadTerrainsDoodads(context.Background(), nil, nil); err != nil {
		t.Fatal(err)
	}
	absolute := func() [4]imath.Vector3 {
		var result [4]imath.Vector3
		m.IterateObjects(func(o *WowObject, a ObjectAbsolute) {
			if o.Type == WowObjectADT {
				if len(o.Model.Liquids) != 1 {
					t.Fatal("liquid metadata lost")
				}
				for i, p := range o.Model.Liquids[0].Cells[0] {
					result[i] = imath.V3Sum(a.Position, imath.V3Rotate(imath.V3Scale(p, a.ScaleFactor), a.Rotation))
				}
			}
		})
		return result
	}
	check := func(got, want imath.Vector3) {
		t.Helper()
		for i := range got {
			if math.Abs(got[i]-want[i]) > 1e-6 {
				t.Fatalf("liquid transform: got %v, want %v", got, want)
			}
		}
	}
	before := absolute()
	for i, p := range source.Cells[0] {
		check(before[i], imath.Vector3{2 * p[1], -2 * p[0], 2 * p[2]})
	}
	// Mutating the model must never mutate the registered source snapshot.
	if snap.Liquids[0].Cells[0] != source.Cells[0] {
		t.Fatal("source liquid snapshot was mutated by recentering")
	}
	ext := ComputeAbsoluteMinMaxExtents(m.Roots)
	center := imath.V3Mean(ext.Min, ext.Max)
	rotation := imath.Vector3{0, 0, imath.Radians(37)}
	m.RotateRootsAtCenter(rotation)
	r.TrimAfterParse()
	if snap.Liquids != nil {
		t.Fatal("parsed snapshot retains liquid payload")
	}
	for i, p := range absolute() {
		check(p, imath.V3Sum(center, imath.V3Rotate(imath.V3Sub(before[i], center), rotation)))
	}
}
