package terrain

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestAdtChunksPreserveGeometryCenterBoundsAndClampIndependentAssets(t *testing.T) {
	model := chunkTestModel()
	original := model.MDL.ToMdl()
	for _, side := range []int{4, 8} {
		parts := splitAdtModel(model, model, side)
		if len(parts) != side*side {
			t.Fatalf("grid %d: %d parts", side, len(parts))
		}
		faces := 0
		for _, part := range parts {
			m := part.model.MDL
			lo, hi := adtVertexBounds(m)
			if chunkVectorLength(imath.V3Sum(lo, hi)) > 1e-7 {
				t.Fatal("part not centered on all axes")
			}
			for _, g := range m.Geosets {
				faces += len(g.Faces)
				for _, v := range g.Vertices {
					p := imath.V3Sum(v.Position, part.center)
					if math.Abs(p[0]-(-30000+16000*v.TexPosition[0])) > 1e-7 || math.Abs(p[1]-(50000+16000*v.TexPosition[1])) > 1e-7 || math.Abs(p[2]-(p[0]+p[1])*.1) > 1e-7 {
						t.Fatal("recenter changed source geometry or UVs")
					}
					if chunkVectorLength(v.Position) > g.BoundsRadius+1e-6 || chunkVectorLength(v.Position) > m.Model.BoundsRadius+1e-6 {
						t.Fatal("vertex outside culling radius")
					}
				}
			}
			parsed := mdx.NewModel()
			if err := parsed.LoadMdl(m.ToMdl()); err != nil {
				t.Fatal(err)
			}
			if parsed.Extent.BoundsRadius > float32(m.Model.BoundsRadius*1.001) {
				t.Fatal("large bounds override applied")
			}
			for _, seq := range m.Sequences {
				if seq.Bound != m.Model.Bound {
					t.Fatal("sequence bounds are stale")
				}
			}
		}
		if faces != 512 {
			t.Fatalf("lost or repeated triangles: %d", faces)
		}
		if model.MDL.ToMdl() != original {
			t.Fatal("splitting mutated source")
		}
	}
	// Real placement path: both clamps use byte-identical local models even
	// though the native heightfield and instance transforms change.
	lo, hi := adtVertexBounds(model.MDL)
	model.MDL.Modify.Translate(imath.V3Negative(imath.V3Mean(lo, hi)))
	var previous []string
	for _, clamp := range []settings.TerrainClampPercent{{Lower: 0, Upper: 1}, {Lower: .2, Upper: .8}} {
		cfg := settings.DefaultMapExportConfig()
		cfg.Terrain.ClampPercent = clamp
		manager := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
		root := &common.WowObject{Type: common.WowObjectADT, Model: model, ScaleFactor: 1, Position: imath.Vector3{100, -200, 50}, Rotation: imath.Vector3{0, 0, .31}}
		manager.Roots = []*common.WowObject{root}
		wc := NewWc3Converter(cfg)
		if err := wc.PrepareAdtChunks(context.Background(), manager, "wow"); err != nil {
			t.Fatal(err)
		}
		mm := extra.NewMapManager()
		mm.SetTerrain(GetInitialTerrain(32, 36))
		if _, err := wc.PlaceDoodads(mm, manager, func(*common.WowObject) bool { return true }); err != nil {
			t.Fatal(err)
		}
		if len(mm.Doodads) != adtChunksPerSide*adtChunksPerSide {
			t.Fatalf("placements=%d", len(mm.Doodads))
		}
		if len(wc.PreloadLocations) != len(mm.Doodads) {
			t.Fatalf("preload stops=%d, want every one of %d chunks", len(wc.PreloadLocations), len(mm.Doodads))
		}
		seenPreloads := map[[2]float32]bool{}
		for i, placed := range mm.Doodads {
			want := [2]float32{placed.Doodad.Position[0], placed.Doodad.Position[1]}
			if wc.PreloadLocations[i] != want || seenPreloads[want] {
				t.Fatalf("chunk %d missing its distinct final camera position: got %v, want %v", i, wc.PreloadLocations[i], want)
			}
			seenPreloads[want] = true
		}
		// Compare actual native scale-then-rotate transforms before/after centering.
		base := NewWc3Converter(cfg)
		before := extra.NewMapManager()
		before.SetTerrain(mm.GetTerrain())
		if _, err := base.PlaceDoodads(before, manager, func(*common.WowObject) bool { return true }); err != nil {
			t.Fatal(err)
		}
		for i, part := range wc.adtChunks[root] {
			p := part.model.MDL.Geosets[0].Vertices[0].Position
			d := mm.Doodads[i].Doodad
			old := before.Doodads[0].Doodad
			native := func(v imath.Vector3, scale [3]float32, position [3]float32) imath.Vector3 {
				for k := range v {
					v[k] *= float64(scale[k])
				}
				v = imath.V3Rotate(v, root.Rotation)
				for k := range v {
					v[k] += float64(position[k])
				}
				return v
			}
			got, want := native(p, d.Scale, d.Position), native(imath.V3Sum(p, part.center), old.Scale, old.Position)
			if chunkVectorLength(imath.V3Sub(got, want)) > .005 {
				t.Fatalf("moved geometry: %v vs %v", got, want)
			}
			text := part.model.MDL.ToMdl()
			if previous != nil && text != previous[i] {
				t.Fatal("full-model assets depend on clamp")
			}
		}
		previous = nil
		for _, part := range wc.adtChunks[root] {
			previous = append(previous, part.model.MDL.ToMdl())
		}
		// Export through the common asset writer: its normal large-model policy
		// must not inflate these terrain extents again.
		out := t.TempDir()
		exportCfg := config.DefaultConfig()
		exportCfg.MDX = false
		am := common.NewAssetManager(exportCfg, nil, nil)
		for key, value := range manager.AssetManager.Models() {
			am.Models()[key] = value
		}
		if err := am.ExportModels(context.Background(), out); err != nil {
			t.Fatal(err)
		}
		part := wc.adtChunks[root][0]
		raw, err := os.ReadFile(filepath.Join(out, part.model.RelativePath+".mdl"))
		if err != nil {
			t.Fatal(err)
		}
		parsed := mdx.NewModel()
		if err := parsed.LoadMdl(string(raw)); err != nil {
			t.Fatal(err)
		}
		if parsed.Extent.BoundsRadius > float32(part.model.MDL.Model.BoundsRadius*1.001) {
			t.Fatal("export inflated chunk bounds")
		}
	}
}

func TestAdtChunksTilesetUsesOnlyClippedFaces(t *testing.T) {
	model := chunkTestModel()
	visual := *model.MDL
	geo := *model.MDL.Geosets[0]
	geo.Faces = append([]components.Face(nil), geo.Faces[:16]...)
	visual.Geosets = []*components.Geoset{&geo}
	manager := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
	root := &common.WowObject{Type: common.WowObjectADT, Model: model, ScaleFactor: 1}
	manager.Roots = []*common.WowObject{root}
	cfg := settings.DefaultMapExportConfig()
	cfg.TerrainAsWc3Tilesets = true
	wc := NewWc3Converter(cfg)
	wc.adtModels = map[*common.WowObject]*common.Model{root: {RelativePath: model.RelativePath + "-tileset-1234", MDL: &visual}}
	if err := wc.PrepareAdtChunks(context.Background(), manager, "wow"); err != nil {
		t.Fatal(err)
	}
	faces := 0
	for _, part := range wc.adtChunks[root] {
		for _, g := range part.model.MDL.Geosets {
			faces += len(g.Faces)
		}
	}
	if faces != 16 {
		t.Fatalf("clipped mesh changed: %d", faces)
	}
}

func chunkTestModel() *common.Model {
	m := mdl.New(mdl.NewMDLOptions{Name: "wow/maps/Test/adt_1_2", FormatVersion: 800})
	tex := &components.Texture{Image: "wow/maps/Test/tex_1_2_0.blp"}
	mat := &components.Material{Layers: []components.Layer{{Texture: tex, Alpha: components.AnimatedOrStatic[float64]{Static: true, Value: 1}}}}
	m.Textures = []*components.Texture{tex}
	m.Materials = []*components.Material{mat}
	bone := components.NewBone("ground")
	m.Bones = []*components.Bone{bone}
	m.Sequences = []components.Sequence{{Name: "Stand", Interval: [2]int{0, 1000}}}
	g := &components.Geoset{Material: mat}
	g.Matrices = []components.Matrix{{Bones: []*components.Bone{bone}}}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			vv := [4]*components.GeosetVertex{}
			for i, xy := range [][2]int{{x, y}, {x + 1, y}, {x + 1, y + 1}, {x, y + 1}} {
				p := imath.Vector3{-30000 + float64(xy[0])*1000, 50000 + float64(xy[1])*1000, 0}
				p[2] = (p[0] + p[1]) * .1
				vv[i] = &components.GeosetVertex{Position: p, Normal: imath.Vector3{0, 0, 1}, TexPosition: imath.Vector2{float64(xy[0]) / 16, float64(xy[1]) / 16}, Matrix: &g.Matrices[0]}
				g.Vertices = append(g.Vertices, vv[i])
			}
			g.Faces = append(g.Faces, components.Face{Vertices: [3]*components.GeosetVertex{vv[0], vv[1], vv[2]}}, components.Face{Vertices: [3]*components.GeosetVertex{vv[0], vv[2], vv[3]}})
		}
	}
	m.Geosets = []*components.Geoset{g}
	m.SyncExtents()
	return &common.Model{RelativePath: m.Model.Name, MDL: m}
}

func chunkVectorLength(v imath.Vector3) float64 { return math.Sqrt(imath.V3Dot(v, v)) }
