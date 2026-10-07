package directm2

import (
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	bundlemeta "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/metadata"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestNormalizeJSONValuesPreservesColorIndexAlphaWhenSiblingHasNaN(t *testing.T) {
	emitter := m2.ParticleEmitterEntry{Position: [3]float32{float32(math.NaN()), 1, 2}}
	track := func(values ...float64) map[string]any {
		return map[string]any{
			"GlobalSeq": uint16(config.BlizzardNull), "Interpolation": uint16(1),
			"Timestamps": [][]uint32{{0}, {0}},
			"Values":     [][][]float64{{{values[0]}}, {{values[1]}}},
		}
	}
	input := map[string]any{
		"fileType":     "m2",
		"m2Animations": []m2.AnimationEntry{{Duration: 1000}, {Duration: 1000}},
		"colors": []map[string]any{{
			"Color": track(1, 1),
			"Alpha": track(0, 32767),
		}},
		"textureTransforms": []m2.TextureTransformEntry{{Translation: m2.Track{Values: [][][]float64{{{0, 1, 0}}}}}},
		"particleEmitters":  []m2.ParticleEmitterEntry{emitter},
	}

	normalized := NormalizeJSONValues(input)
	data, ok := normalized.(map[string]any)
	if !ok {
		t.Fatalf("normalized metadata type = %T, want map[string]any", normalized)
	}
	if _, ok := data["colors"].([]any); !ok {
		t.Fatalf("normalized colors type = %T, want []any", data["colors"])
	}
	if _, ok := data["textureTransforms"].([]any); !ok {
		t.Fatalf("normalized texture transforms type = %T, want []any", data["textureTransforms"])
	}
	emitters, ok := data["particleEmitters"].([]any)
	if !ok {
		t.Fatalf("normalized particle emitters type = %T, want []any", data["particleEmitters"])
	}
	position := emitters[0].(map[string]any)["position"].([]any)
	if position[0] != nil {
		t.Fatalf("non-finite source position = %#v, want JSON null", position[0])
	}
	if !math.IsNaN(float64(emitter.Position[0])) {
		t.Fatal("normalization mutated the source particle metadata")
	}

	meta := bundlemeta.NewFile("", config.Config{}, nil)
	meta.LoadFromData(data)
	geosetAnim := meta.GeosetAnimation(0, &components.Geoset{})
	if geosetAnim.Alpha == nil || geosetAnim.Alpha.Static || geosetAnim.Alpha.Anim == nil {
		t.Fatalf("ColorIndex alpha track = %+v, want a sequence animation", geosetAnim.Alpha)
	}
	if got := geosetAnim.Alpha.Anim.KeyFrames[0]; got != float64(0) {
		t.Fatalf("Stand alpha at first key = %#v, want source zero", got)
	}
}
