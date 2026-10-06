package directwmo

import (
	"context"
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

func TestWmoBakeWorkersPreserveOutput(t *testing.T) {
	for _, animated := range []bool{false, true} {
		t.Run(map[bool]string{false: "still", true: "animated"}[animated], func(t *testing.T) {
			serial := syntheticWmoBakeFixture(0)
			parallel := syntheticWmoBakeFixture(0)
			cfg := config.DefaultConfig()
			cfg.TextureBaking.Animate = &animated
			cfg.ExportAssetDir = t.TempDir()

			if err := bakeWmoMaterialsWithWorkers(context.Background(), cfg, nil, serial.root, serial.groups, &serial.result, 1); err != nil {
				t.Fatal(err)
			}
			if err := bakeWmoMaterialsWithWorkers(context.Background(), cfg, nil, parallel.root, parallel.groups, &parallel.result, 2); err != nil {
				t.Fatal(err)
			}
			serial.result.MDL.Modify.OptimizeAll()
			parallel.result.MDL.Modify.OptimizeAll()
			serialText, parallelText := serial.result.MDL.ToMdl(), parallel.result.MDL.ToMdl()
			if got, want := parallelText, serialText; got != want {
				mismatch := firstStringMismatch(got, want)
				start, end := max(0, mismatch-120), min(len(got), mismatch+240)
				t.Fatalf("parallel output MDL differs from serial output at byte %d\nparallel: %q\nserial:   %q", mismatch, got[start:end], want[start:min(len(want), end)])
			}
			serialReloaded, parallelReloaded := mdx.NewModel(), mdx.NewModel()
			if err := serialReloaded.LoadMdl(serialText); err != nil {
				t.Fatalf("reload serial MDL: %v", err)
			}
			if err := parallelReloaded.LoadMdl(parallelText); err != nil {
				t.Fatalf("reload parallel MDL: %v", err)
			}
			if got, want := parallelReloaded.SaveMdx(), serialReloaded.SaveMdx(); !reflect.DeepEqual(got, want) {
				t.Fatalf("reloaded parallel MDL differs from reloaded serial MDL")
			}
			if got, want := registeredPNGBytes(t, &parallel.result), registeredPNGBytes(t, &serial.result); !reflect.DeepEqual(got, want) {
				t.Fatalf("parallel output PNGs differ from serial output")
			}
		})
	}
}

func TestWmoBakeWorkersPreserveMultipleAnimatedOutputs(t *testing.T) {
	serial := syntheticWmoBakeFixture(3)
	parallel := syntheticWmoBakeFixture(3)
	cfg := config.DefaultConfig()
	animated := true
	cfg.TextureBaking.Animate = &animated
	cfg.ExportAssetDir = t.TempDir()
	if err := bakeWmoMaterialsWithWorkers(context.Background(), cfg, nil, serial.root, serial.groups, &serial.result, 1); err != nil {
		t.Fatal(err)
	}
	if err := bakeWmoMaterialsWithWorkers(context.Background(), cfg, nil, parallel.root, parallel.groups, &parallel.result, 2); err != nil {
		t.Fatal(err)
	}
	serial.result.MDL.Modify.OptimizeAll()
	parallel.result.MDL.Modify.OptimizeAll()
	serialText, parallelText := serial.result.MDL.ToMdl(), parallel.result.MDL.ToMdl()
	if parallelText != serialText {
		t.Fatalf("parallel multi-output animated MDL differs from serial MDL")
	}
	serialReloaded, parallelReloaded := mdx.NewModel(), mdx.NewModel()
	if err := serialReloaded.LoadMdl(serialText); err != nil {
		t.Fatalf("reload serial MDL: %v", err)
	}
	if err := parallelReloaded.LoadMdl(parallelText); err != nil {
		t.Fatalf("reload parallel MDL: %v", err)
	}
	if !reflect.DeepEqual(parallelReloaded.SaveMdx(), serialReloaded.SaveMdx()) {
		t.Fatal("reloaded parallel multi-output animated MDL differs from serial MDL")
	}
	if !reflect.DeepEqual(registeredPNGBytes(t, &parallel.result), registeredPNGBytes(t, &serial.result)) {
		t.Fatal("parallel multi-output animated PNGs differ from serial PNGs")
	}
}

func TestWmoMergeRebasesMultiAnimationOutputsOnce(t *testing.T) {
	model := mdl.New(mdl.NewMDLOptions{FormatVersion: 1000, Name: "animation-merge-fixture"})
	model.TextureAnims = []components.TextureAnim{{ID: 0}}
	model.Geosets = []*components.Geoset{{Name: "base0"}, {Name: "base1"}}
	model.Materials = []*components.Material{{}}
	destination := directm2.ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	baseTextureAnims := append([]components.TextureAnim(nil), model.TextureAnims...)
	baseGeosets := append([]*components.Geoset(nil), model.Geosets...)
	baseMaterials := append([]*components.Material(nil), model.Materials...)

	mergeAnimatedWorker := func(geosetIndex, newAnimationCount int) {
		workerModel := *model
		workerModel.TextureAnims = append([]components.TextureAnim(nil), baseTextureAnims...)
		workerModel.TextureAnims = workerModel.TextureAnims[:len(workerModel.TextureAnims):len(workerModel.TextureAnims)]
		material := &components.Material{}
		for animationIndex := range newAnimationCount {
			localID := len(baseTextureAnims) + animationIndex
			workerModel.TextureAnims = append(workerModel.TextureAnims, components.TextureAnim{ID: localID})
			// BakeM2Geoset stores a pointer to its local TextureAnim value in
			// material layers; that pointer is distinct from the slice copy.
			material.Layers = append(material.Layers, components.Layer{TVertexAnim: &components.TextureAnim{ID: localID}})
		}
		workerModel.Geosets = append([]*components.Geoset(nil), baseGeosets...)
		workerModel.Geosets[geosetIndex] = &components.Geoset{Name: "baked", Material: material}
		workerModel.Materials = append([]*components.Material(nil), baseMaterials...)
		workerModel.Materials = append(workerModel.Materials, material)
		worker := directm2.ConvertResult{MDL: &workerModel, TexturePaths: map[string]struct{}{}}
		task := wmoBakeTask{
			geosetIndex: geosetIndex, beforeGeosetCount: len(model.Geosets),
			baseMaterialCount: len(baseMaterials), baseTextureCount: len(model.Textures),
			baseAnimCount: len(baseTextureAnims), baseGlobalCount: len(model.GlobalSequences),
		}
		if err := mergeWmoBakeResult(&destination, worker, task); err != nil {
			t.Fatal(err)
		}
	}

	// The first batch contributes one animation, while the next contributes
	// two separate animated texture outputs. Its destination offset is within
	// the worker's local animation range, exposing repeated local-ID rebasing.
	mergeAnimatedWorker(0, 1)
	mergeAnimatedWorker(1, 2)
	rebaseWmoTextureAnimations(destination.MDL, len(baseMaterials))

	wantIDs := [][]int{{1}, {2, 3}}
	for materialIndex, ids := range wantIDs {
		material := destination.MDL.Materials[materialIndex+1]
		for layerIndex, wantID := range ids {
			animation := material.Layers[layerIndex].TVertexAnim
			if animation != &destination.MDL.TextureAnims[wantID] {
				t.Fatalf("material %d layer %d points to animation ID %d; want %d", materialIndex+1, layerIndex, animation.ID, wantID)
			}
		}
	}
}

func firstStringMismatch(a, b string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

type syntheticWmoBake struct {
	root   *wmo.Loader
	groups []*wmo.Loader
	result directm2.ConvertResult
}

func syntheticWmoBakeFixture(shader uint32) syntheticWmoBake {
	const batches = 4
	model := mdl.New(mdl.NewMDLOptions{FormatVersion: 1000, Name: "worker-fixture"})
	model.Textures = []*components.Texture{{ID: 0}}
	model.TextureAnims = []components.TextureAnim{{ID: 0}}
	group := &wmo.Loader{}
	root := &wmo.Loader{Materials: make([]wmo.Material, batches)}
	root.MaterialUVSpeed = make([][4]float32, batches)
	result := directm2.ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}, BakeStem: "worker-fixture"}
	for batchIndex := range batches {
		root.Materials[batchIndex] = wmo.Material{Shader: shader, BlendMode: 4}
		root.MaterialUVSpeed[batchIndex] = [4]float32{float32(100 + batchIndex*37), float32(25 + batchIndex*19)}
		group.RenderBatches = append(group.RenderBatches, wmo.RenderBatch{FirstFace: uint32(batchIndex * 3), NumFaces: 3, MaterialID: uint8(batchIndex)})
		geoset := &components.Geoset{Name: "batch", Matrices: []components.Matrix{{ID: 0}}, Material: &components.Material{Layers: []components.Layer{{Texture: model.Textures[0], TVertexAnim: &model.TextureAnims[0]}}}}
		for corner := range 3 {
			index := batchIndex*3 + corner
			group.Indices = append(group.Indices, uint16(index))
			x, y := float32(corner%2), float32(corner/2)
			group.Vertices = append(group.Vertices, x+float32(batchIndex)*2, y, 0)
			group.Normals = append(group.Normals, 0, 0, 1)
			group.UVs = ensureUVLayer(group.UVs, 1)
			group.UVs[0] = append(group.UVs[0], x, y)
			vertex := &components.GeosetVertex{Position: imath.Vector3{float64(x), float64(y), 0}, Normal: imath.Vector3{0, 0, 1}, TexPosition: imath.Vector2{float64(x), float64(y)}, Matrix: &geoset.Matrices[0]}
			geoset.Vertices = append(geoset.Vertices, vertex)
		}
		geoset.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{geoset.Vertices[0], geoset.Vertices[1], geoset.Vertices[2]}}}
		model.Geosets = append(model.Geosets, geoset)
	}
	return syntheticWmoBake{root: root, groups: []*wmo.Loader{group}, result: result}
}

func ensureUVLayer(layers [][]float32, count int) [][]float32 {
	for len(layers) < count {
		layers = append(layers, nil)
	}
	return layers
}

func registeredPNGBytes(t *testing.T, result *directm2.ConvertResult) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte, len(result.TexturePaths))
	for path := range result.TexturePaths {
		source, ok := texturesource.Get(path)
		if !ok || source.Kind != texturesource.KindPNG {
			t.Fatalf("baked PNG %q is not registered", path)
		}
		out[path] = append([]byte(nil), source.PNG...)
	}
	return out
}
