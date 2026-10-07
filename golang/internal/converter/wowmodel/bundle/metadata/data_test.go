package metadata

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

func TestTypedMetadataOwnsNestedTracksAndSanitizesNonfiniteValues(t *testing.T) {
	native := m2.Track{GlobalSeq: 65535, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{math.NaN(), 2, 3}}}}
	uv := Track{GlobalSeq: 0, Timestamps: [][]*uint32{{timestamp(0), nil}}, Values: [][][]float64{{{1, 2, 3}}}}
	source := Data{
		FileType: "m2", TextureTransforms: []TextureTransform{{Translation: uv}},
		Colors: []Color{{Color: uv}}, TextureWeights: []m2.Track{native},
		Cameras:        []m2.CameraEntry{{FoV: native, FarClip: float32(math.Inf(1))}},
		Lights:         []m2.LightEntry{{DiffuseColor: native}},
		RibbonEmitters: []m2.RibbonEmitterEntry{{TextureIndices: []uint16{4}, ColorTrack: native}},
		ParticleEmitters: []m2.ParticleEmitterEntry{{
			Position: [3]float32{float32(math.NaN()), 2, 3}, EmissionSpeed: native,
			ColorTrack: m2.PartTrack{Timestamps: []uint16{10}, Values: [][]float64{{math.Inf(-1), 2, 3}}},
		}},
		WMOMaterials: []wmo.Material{{RuntimeData: []uint32{12}}},
	}
	f := NewFile("", config.Config{}, nil)
	f.LoadFromData(source)
	if f.textureWeights[0].Values[0][0][0] != 0 || f.cameras[0].FarClip != 0 || f.lights[0].DiffuseColor.Values[0][0][0] != 0 || f.ribbonEmitters[0].ColorTrack.Values[0][0][0] != 0 || f.particleEmitters[0].Position[0] != 0 || f.particleEmitters[0].ColorTrack.Values[0][0] != 0 {
		t.Fatal("non-finite source values survived the typed boundary")
	}
	if !math.IsNaN(native.Values[0][0][0]) || !math.IsNaN(float64(source.ParticleEmitters[0].Position[0])) {
		t.Fatal("snapshot sanitation mutated source values")
	}
	*uv.Timestamps[0][0] = 99
	uv.Values[0][0][0] = 99
	native.Timestamps[0][0] = 99
	native.Values[0][0][1] = 99
	source.RibbonEmitters[0].TextureIndices[0] = 99
	source.ParticleEmitters[0].ColorTrack.Values[0][1] = 99
	source.WMOMaterials[0].RuntimeData[0] = 99
	if *f.textureTransforms[0].Translation.Timestamps[0][0] != 0 || f.textureTransforms[0].Translation.Timestamps[0][1] != nil || f.colors[0].Color.Values[0][0][0] != 1 {
		t.Fatal("UV/color tracks retained caller-owned pointers or slices")
	}
	if f.textureWeights[0].Timestamps[0][0] != 0 || f.cameras[0].FoV.Values[0][0][1] != 2 || f.lights[0].DiffuseColor.Values[0][0][1] != 2 || f.ribbonEmitters[0].TextureIndices[0] != 4 || f.particleEmitters[0].ColorTrack.Values[0][1] != 2 || f.wmoMaterials[0].RuntimeData[0] != 12 {
		t.Fatal("native emitter/material tracks retained caller-owned slices")
	}
	f.LoadFromData(Data{FileType: "wmo"})
	if !f.IsWmo() || len(f.colors) != 0 || len(f.particleEmitters) != 0 || len(f.wmoMaterials) != 0 {
		t.Fatal("reload accumulated metadata from the previous model")
	}
}

func TestTypedGlobalUVAndStaticBGRColor(t *testing.T) {
	f := NewFile("", config.Config{}, nil)
	f.LoadFromData(Data{
		FileType: "m2", GlobalLoops: []uint32{1000},
		TextureTransforms: []TextureTransform{{Translation: Track{GlobalSeq: 0, Interpolation: 1, Timestamps: [][]*uint32{{timestamp(0), timestamp(500)}}, Values: [][][]float64{{{0, 0, 0}, {1, 2, 0}}}}}},
		Colors:            []Color{{Color: Track{Values: [][][]float64{{{0.1, 0.2, 0.3}}}}, Alpha: Track{Values: [][][]float64{{{16383.5}}}}}},
	})
	model := &mdl.MDL{}
	f.BindMdl(model)
	anims := f.buildTextureAnims()
	if len(anims) != 1 || anims[0].Translation == nil || anims[0].Translation.GlobalSeq.Duration != 1000 || anims[0].Translation.KeyFrames[500] != (imath.Vector3{1, 2, 0}) {
		t.Fatalf("global-only UV animation lost: %#v", anims)
	}
	ga := f.GeosetAnimation(0, &components.Geoset{})
	if ga.Color == nil || !ga.Color.Static || ga.Color.Value != (imath.Vector3{0.3, 0.2, 0.1}) || ga.Alpha == nil || !ga.Alpha.Static || ga.Alpha.Value != 0.5 {
		t.Fatalf("static BGR color/alpha changed: %#v", ga)
	}
}

func TestParseLegacyNullableTrackAndNoGlobalDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.json")
	content := `{"fileType":"m2","m2Animations":[{"duration":1000}],"colors":[{"alpha":{"timestamps":[[null,100]],"values":[[[0],[32767]]]}}],"skin":{"subMeshes":[{"enabled":true}],"textureUnits":[{"priority":-3}]}}`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	f := NewFile(path, config.Config{}, nil)
	if err := f.Parse(); err != nil {
		t.Fatal(err)
	}
	if f.colors[0].Alpha.GlobalSeq != 65535 || f.colors[0].Alpha.Timestamps[0][0] != nil || f.skin.TextureUnits[0].Priority != -3 {
		t.Fatal("legacy nullable/default/signed metadata changed")
	}
	ga := f.GeosetAnimation(0, &components.Geoset{})
	if ga.Alpha == nil || ga.Alpha.Anim == nil || ga.Alpha.Anim.GlobalSeq != nil || len(ga.Alpha.Anim.KeyFrames) != 2 || ga.Alpha.Anim.KeyFrames[100] != float64(1) || ga.Alpha.Anim.KeyFrames[1000] != float64(1) {
		t.Fatalf("legacy track = %#v", ga.Alpha)
	}
}

func TestParseWMOMaterialsAndRejectMalformedTypedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wmo.json")
	if err := os.WriteFile(path, []byte(`{"fileType":"wmo","materials":[{"flags":5,"texture1":12,"runtimeData":[13]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	f := NewFile(path, config.Config{}, nil)
	if err := f.Parse(); err != nil {
		t.Fatal(err)
	}
	if !f.IsWmo() || len(f.wmoMaterials) != 1 || f.wmoMaterials[0].Texture1 != 12 || f.wmoMaterials[0].RuntimeData[0] != 13 {
		t.Fatalf("WMO materials = %#v", f.wmoMaterials)
	}
	if err := os.WriteFile(path, []byte(`{"fileType":"m2","materials":[{"flags":"bad"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.Parse(); err == nil {
		t.Fatal("malformed typed material accepted")
	}
}
