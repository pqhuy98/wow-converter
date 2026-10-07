package directm2

import (
	"github.com/pqhuy98/wow-converter/internal/config"
	bundlemeta "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/metadata"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
	"math"
	"testing"
)

func TestTypedMetadataPreservesColorAlphaWhenSiblingHasNaN(t *testing.T) {
	track := func(values ...float64) m2.Track {
		return m2.Track{GlobalSeq: uint16(config.BlizzardNull), Interpolation: 1, Timestamps: [][]uint32{{0}, {0}}, Values: [][][]float64{{{values[0]}}, {{values[1]}}}}
	}
	loader := &m2.Loader{
		Animations:        []m2.AnimationEntry{{Duration: 1000}, {Duration: 1000}},
		Colors:            []m2.ColorEntry{{Color: track(1, 1), Alpha: track(0, 32767)}},
		TextureTransforms: []m2.TextureTransformEntry{{Translation: m2.Track{GlobalSeq: uint16(config.BlizzardNull), Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{0, 1, 0}}}}}},
		ParticleEmitters:  []m2.ParticleEmitterEntry{{Position: [3]float32{float32(math.NaN()), 1, 2}}},
	}
	meta := bundlemeta.NewFile("", config.Config{}, nil)
	meta.LoadFromData(buildMetadata(loader, &m2.Skin{}, nil, nil, nil))
	geosetAnim := meta.GeosetAnimation(0, &components.Geoset{})
	if geosetAnim.Alpha == nil || geosetAnim.Alpha.Static || geosetAnim.Alpha.Anim == nil {
		t.Fatalf("alpha = %+v, want sequence animation", geosetAnim.Alpha)
	}
	if got := geosetAnim.Alpha.Anim.KeyFrames[0]; got != float64(0) {
		t.Fatalf("first alpha = %#v, want zero", got)
	}
	if !math.IsNaN(float64(loader.ParticleEmitters[0].Position[0])) {
		t.Fatal("metadata changed the source particle")
	}
}
