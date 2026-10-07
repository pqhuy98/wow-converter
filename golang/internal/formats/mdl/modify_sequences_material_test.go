package mdl

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestDecayKeepsMaterialAndUVTracksOnSameTimeline(t *testing.T) {
	m := New(NewMDLOptions{Name: "paged effect"})
	m.Sequences = []components.Sequence{
		{Name: "Death", Interval: [2]int{0, 100}},
		{Name: "Stand", Interval: [2]int{101, 201}},
	}
	first, last := &components.Texture{Image: "first.blp"}, &components.Texture{Image: "last.blp"}
	pages := &components.Animation{Type: components.AnimTypeOthers, KeyFrames: map[int]any{0: first, 100: last, 101: first, 201: last}}
	opacity := &components.Animation{Type: components.AnimTypeAlpha, KeyFrames: map[int]any{0: 1.0, 100: 0.4, 101: 0.8, 201: 1.0}}
	uv := &components.Animation{Type: components.AnimTypeTVertexAnim, KeyFrames: map[int]any{0: imath.Vector3{}, 100: imath.Vector3{1, 0, 0}, 101: imath.Vector3{}, 201: imath.Vector3{1, 0, 0}}}
	gs := components.NewGlobalSequence(0, 500)
	global := &components.Animation{GlobalSeq: &gs, KeyFrames: map[int]any{0: first, 200: last}}
	light := &components.Animation{KeyFrames: map[int]any{0: 1.0, 100: 0.2, 101: 0.7, 201: 1.0}}
	// A flying exhaust can emit during Death but be off during a later Stand.
	// Leaving its keys behind makes the renderer fall back to the wrong state.
	emission := &components.Animation{KeyFrames: map[int]any{0: 50.0, 100: 50.0, 101: 0.0, 201: 0.0}}
	visibility := &components.Animation{KeyFrames: map[int]any{0: 1.0, 100: 1.0, 101: 1.0, 201: 1.0}}
	m.ParticleEmitter2s = []*components.ParticleEmitter2{{EmissionRate: components.AnimatedOrStatic[float64]{Anim: emission}, Visibility: visibility}}
	m.Lights = []*components.Light{{Intensity: components.AnimatedOrStatic[float64]{Anim: light}}}
	m.TextureAnims = []components.TextureAnim{{Translation: uv}}
	m.Materials = []*components.Material{{Layers: []components.Layer{
		{TextureIDAnim: pages, Alpha: components.AnimatedOrStatic[float64]{Anim: opacity}},
		{TextureIDAnim: global, Alpha: components.AnimatedOrStatic[float64]{Static: true, Value: 1}},
	}}}
	m.Modify.AddDecayAnimation()

	stand := m.Sequences[1]
	if stand.Interval != [2]int{120103, 120203} {
		t.Fatalf("Stand interval = %v", stand.Interval)
	}
	if pages.KeyFrames[stand.Interval[0]] != first || pages.KeyFrames[stand.Interval[1]] != last {
		t.Fatal("texture page keys no longer cover the shifted Stand sequence")
	}
	if opacity.KeyFrames[stand.Interval[0]] != 0.8 || uv.KeyFrames[stand.Interval[1]] != (imath.Vector3{1, 0, 0}) {
		t.Fatal("material opacity and UV keys no longer share the page timeline")
	}
	for _, timestamp := range stand.Interval {
		if emission.KeyFrames[timestamp] != 0.0 || visibility.KeyFrames[timestamp] != 1.0 {
			t.Fatal("particle state did not move with its Stand interval")
		}
	}
	for _, decay := range m.Sequences[2:] {
		for _, timestamp := range decay.Interval {
			if pages.KeyFrames[timestamp] != last || opacity.KeyFrames[timestamp] != 0.4 || light.KeyFrames[timestamp] != 0.2 {
				t.Fatalf("death material state missing at decay boundary %d", timestamp)
			}
			if visibility.KeyFrames[timestamp] != 0.0 {
				t.Fatal("decay must suppress particles without changing their other sequences")
			}
		}
	}
	if len(global.KeyFrames) != 2 || global.KeyFrames[200] != last {
		t.Fatal("independent global clock was shifted")
	}
}
