package metadata

import (
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestParticleGravityDecodesStaticAndAnimatedTracks(t *testing.T) {
	for _, vector := range []bool{false, true} {
		flags := uint32(0x20000)
		bits, want := math.Float32bits(2.5), 2.5
		if vector {
			flags |= 0x00800000
			bits, want = uint32(uint16(65536-200))<<16, 200*0.04238648
		}
		for _, animated := range []bool{false, true} {
			track := m2.Track{GlobalSeq: config.BlizzardNull, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{float64(bits)}}}}
			if animated {
				track.Timestamps = append(track.Timestamps, []uint32{0})
				track.Values = append(track.Values, [][]float64{{float64(bits)}})
			}
			f := &File{IsLoaded: true, m2Animations: []m2AnimMeta{{Duration: 500}, {Duration: 500}}, particleEmitters: []m2.ParticleEmitterEntry{{Flags: flags, Gravity: track}}}
			model := mdl.New(mdl.NewMDLOptions{Name: "gravity"})
			model.Bones = []*components.Bone{components.NewBone("root")}
			f.BindMdl(model)
			f.ExtractMDLParticlesEmitters([]components.Texture{{Image: "test.blp"}})
			gravity := model.ParticleEmitter2s[0].Gravity
			if !animated {
				if !gravity.Static || math.Abs(gravity.Value-want) > 1e-6 {
					t.Fatalf("static vector=%v: %v", vector, gravity)
				}
			} else {
				for _, value := range gravity.Anim.KeyFrames {
					if math.Abs(value.(float64)-want) > 1e-6 {
						t.Fatalf("animated vector=%v: %v want %v", vector, value, want)
					}
				}
			}
		}
	}
}

func TestParticleTailDurationFollowsSourceFlags(t *testing.T) {
	for _, test := range []struct {
		tail  float32
		flags uint32
		want  float64
	}{
		{0, 0x40000, 0}, {3, 0x40000, 3}, {3, 0x40400, 1}, {1, 0x40400, 0.75},
	} {
		p := m2.ParticleEmitterEntry{Flags: test.flags, TailLength: test.tail, Lifespan: m2.Track{Values: [][][]float64{{{2}}}}}
		_, got, _ := correctParticleEmission(p, components.AnimatedOrStatic[float64]{Static: true, Value: 70})
		if math.Abs(got-test.want) > 1e-6 {
			t.Fatalf("tail=%v flags=%x: got %v want %v", test.tail, test.flags, got, test.want)
		}
	}
}

func TestParticleAgeCurvesInterpolateAndClamp(t *testing.T) {
	p := m2.ParticleEmitterEntry{
		AlphaTrack: m2.PartTrack{Timestamps: []uint16{0, 9830, 32767}, Values: [][]float64{{0}, {32767}, {0}}},
		ScaleTrack: m2.PartTrack{Timestamps: []uint16{0, 32767}, Values: [][]float64{{1, 1}, {2, 2}}},
	}
	p.TwinkleScale.Min, p.TwinkleScale.Max = 1, 1
	want := 1 + float64(9830)/32767
	if got := particleSegmentScaling(p); got != [3]float64{1, want, 2} {
		t.Fatalf("size jumped to end key at alpha peak: %v", got)
	}
	if got := partValueAt(m2.PartTrack{Timestamps: []uint16{0}, Values: [][]float64{{3}}}, 32767); len(got) != 1 || got[0] != 3 {
		t.Fatal("single key should stay constant")
	}
	node := &components.ParticleEmitter2{Rows: 2, Columns: 2, HeadIntervals: [3]float64{2, 4, 1}}
	clampParticleUVIntervals(node)
	if node.HeadIntervals != [3]float64{2, 4, 1} {
		t.Fatal("linear atlas indices were clamped to row/column counts")
	}
}

func TestParticlePlanarAndBillboardFlags(t *testing.T) {
	for _, flags := range []uint32{0x31411, 0x30031} {
		f := &File{IsLoaded: true, particleEmitters: []m2.ParticleEmitterEntry{{Flags: flags}}}
		model := mdl.New(mdl.NewMDLOptions{Name: "particle orientation"})
		model.Bones = []*components.Bone{components.NewBone("root")}
		f.BindMdl(model)
		f.ExtractMDLParticlesEmitters([]components.Texture{{Image: "test.blp"}})
		planar := false
		for _, flag := range model.ParticleEmitter2s[0].Flags2 {
			planar = planar || flag == components.PE2XYQuad
		}
		if planar != (flags&0x1000 != 0) {
			t.Fatalf("flags %x: planar=%v", flags, planar)
		}
	}
}

func TestParticleVisibilitySparseAnimations(t *testing.T) {
	f := &File{m2Animations: []m2AnimMeta{{Duration: 67}, {Duration: 9334}, {Duration: 60000}, {Duration: 9334}, {Duration: 33}}}
	track := m2.Track{GlobalSeq: config.BlizzardNull,
		Timestamps: [][]uint32{{0}, {}, {}, {}, {0}},
		Values:     [][][]float64{{{0}}, {}, {}, {}, {{0}}},
	}
	anim := f.particleVisibility(track)
	for frame, want := range map[int]float64{0: 0, 67: 0, 68: 1, 9402: 1, 9403: 1, 69403: 1, 69404: 1, 78738: 1, 78739: 0} {
		if got := anim.KeyFrames[frame]; got != want {
			t.Errorf("frame %d: got %v, want %v", frame, got, want)
		}
	}
	if len(track.Values[1]) != 0 {
		t.Fatal("source visibility mutated")
	}
	global := m2.Track{GlobalSeq: 0, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{0}}}}
	if got := f.particleVisibility(global); got.GlobalSeq == nil || len(got.KeyFrames) != 1 || got.KeyFrames[0] != float64(0) {
		t.Fatal("global visibility changed")
	}
}

func TestParticleEmissionPreservesSourceRatesAndDisabledKeys(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags uint32
		rate  float64
	}{
		{"banner circle disabled", 133672, 0},
		{"banner ring disabled", 135208, 0},
		{"disabled tail", 0x40400, 0},
		{"head sprite", 0x20400, 1},
		{"active tail", 0x40400, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := m2.ParticleEmitterEntry{Flags: tc.flags, TailLength: 0.1,
				Lifespan:     m2.Track{Values: [][][]float64{{{1.5}}}},
				EmissionRate: m2.Track{Values: [][][]float64{{{tc.rate}}}},
				AlphaTrack:   m2.PartTrack{Values: [][]float64{{0}, {12850}, {0}}},
			}
			_, _, rate := correctParticleEmission(p, components.AnimatedOrStatic[float64]{Static: true, Value: tc.rate})
			if rate.Value != tc.rate {
				t.Fatalf("emission changed from %v to %v", tc.rate, rate.Value)
			}
			anim := &components.Animation{KeyFrames: map[int]any{0: float64(0), 100: tc.rate}}
			_, _, animated := correctParticleEmission(p, components.AnimatedOrStatic[float64]{Anim: anim})
			if animated.Anim.KeyFrames[0] != float64(0) {
				t.Fatal("disabled emission key was boosted")
			}
		})
	}
}

func TestExtractMDLParticlesEmittersBasic(t *testing.T) {
	f := &File{
		IsLoaded: true,
		particleEmitters: []m2.ParticleEmitterEntry{{
			ParticleID: 1, Bone: 0, TexturePacked: 0, TextureRows: 1, TextureCols: 1,
			Position: [3]float32{1, 3, -2},
		}},
	}
	model := mdl.New(mdl.NewMDLOptions{Name: "test"})
	model.Bones = []*components.Bone{components.NewBone("root")}
	f.BindMdl(model)
	textures := []components.Texture{{Image: "wow/spells/tex0.blp", WowData: components.TextureWowData{PngPath: "spells/tex0.png"}}}
	f.ExtractMDLParticlesEmitters(textures)
	if len(model.ParticleEmitter2s) != 1 {
		t.Fatalf("expected 1 particle emitter, got %d", len(model.ParticleEmitter2s))
	}
	if got := model.ParticleEmitter2s[0].PivotPoint; got != (imath.Vector3{1, 2, 3}) {
		t.Fatalf("particle pivot %v, want model-space bind point (1,2,3)", got)
	}
}

func TestExtractMDLParticlesEmittersRefractionRouting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags uint32
		want  int
	}{
		{"ordinary gas", 0x20001, 1},
		{"scene refraction", 0x00120001, 0},
		{"multitexture takes precedence", 0x10120001, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &File{IsLoaded: true, particleEmitters: []m2.ParticleEmitterEntry{{Flags: tc.flags}}}
			model := mdl.New(mdl.NewMDLOptions{Name: "particle shader routing"})
			model.Bones = []*components.Bone{components.NewBone("root")}
			f.BindMdl(model)
			f.ExtractMDLParticlesEmitters([]components.Texture{{Image: "test.blp"}})
			if got := len(model.ParticleEmitter2s); got != tc.want {
				t.Fatalf("got %d visible emitters, want %d", got, tc.want)
			}
		})
	}
}

func TestExtractMDLParticlesEmittersModelSpacePivots(t *testing.T) {
	for _, tc := range []struct {
		name         string
		parent, want imath.Vector3
		loaded       [3]float32
	}{
		{"nonzero offset", imath.Vector3{10, 20, 30}, imath.Vector3{11, 22, 33}, [3]float32{11, 33, -22}},
		{"model origin", imath.Vector3{10, 20, 30}, imath.Vector3{}, [3]float32{}},
		{"mirrored point", imath.Vector3{1, 2, 3}, imath.Vector3{-1, 2, 3}, [3]float32{-1, 3, -2}},
		{"point on parent", imath.Vector3{1, 2, 3}, imath.Vector3{1, 2, 3}, [3]float32{1, 3, -2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &File{IsLoaded: true, particleEmitters: []m2.ParticleEmitterEntry{{Position: tc.loaded, Bone: 0}}}
			model := mdl.New(mdl.NewMDLOptions{Name: "particle bind point"})
			parent := components.NewBone("parent")
			parent.PivotPoint = tc.parent
			model.Bones = []*components.Bone{parent}
			f.BindMdl(model)
			f.ExtractMDLParticlesEmitters([]components.Texture{{Image: "test.blp"}})
			if len(model.ParticleEmitter2s) != 1 {
				t.Fatal("expected one emitter")
			}
			emitter := model.ParticleEmitter2s[0]
			if emitter.PivotPoint != tc.want || emitter.Parent != parent {
				t.Fatalf("pivot %v, parent %p; want pivot %v, parent %p", emitter.PivotPoint, emitter.Parent, tc.want, parent)
			}
		})
	}
}

func TestExtractMDLRibbonsPreservesBindPointsAndBoneFrame(t *testing.T) {
	for _, tc := range []struct {
		name         string
		parent, want imath.Vector3
		position     [3]float32
	}{
		{"point on wing bone", imath.Vector3{1, 2, 3}, imath.Vector3{1, 2, 3}, [3]float32{1, 2, 3}},
		{"offset from bone", imath.Vector3{10, 20, 30}, imath.Vector3{11, 22, 33}, [3]float32{11, 22, 33}},
		{"mirrored point", imath.Vector3{1, 2, 3}, imath.Vector3{-1, 2, 3}, [3]float32{-1, 2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &File{IsLoaded: true, materials: []m2.MaterialEntry{{BlendingMode: 2}}, ribbonEmitters: []m2.RibbonEmitterEntry{{BoneIndex: 0, Position: tc.position, TextureIndices: []uint16{0}, MaterialIndices: []uint16{0}}}}
			m := mdl.New(mdl.NewMDLOptions{Name: "ribbon bind point"})
			parent := components.NewBone("wing")
			parent.PivotPoint = tc.parent
			parent.Rotation = &components.Animation{Type: components.AnimTypeRotation, KeyFrames: map[int]any{0: imath.QuaternionRotation{0.5, 0.5, 0.5, 0.5}}}
			m.Bones = []*components.Bone{parent}
			f.BindMdl(m)
			f.ExtractMDLRibbonEmitters([]components.Texture{{Image: "ribbon.blp"}})
			if len(m.RibbonEmitters) != 1 {
				t.Fatal("ribbon missing")
			}
			ribbon := m.RibbonEmitters[0]
			if ribbon.Parent != parent || ribbon.PivotPoint != tc.want {
				t.Fatalf("ribbon pivot %v, parent %p; want pivot %v, parent %p", ribbon.PivotPoint, ribbon.Parent, tc.want, parent)
			}
			if ribbon.Rotation != nil {
				t.Fatal("ribbon should inherit the bone frame without an extra basis rotation")
			}
		})
	}
}

func TestExtractMDLLightsTypeMapping(t *testing.T) {
	tests := []struct {
		name string
		raw  uint16
		want components.LightType
	}{
		{name: "raw 0 becomes directional", raw: 0, want: components.LightDirectional},
		{name: "raw 1 becomes omnidirectional", raw: 1, want: components.LightOmnidirectional},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{
				IsLoaded: true,
				lights: []m2.LightEntry{
					{Type: tt.raw},
				},
			}

			model := mdl.New(mdl.NewMDLOptions{Name: "test"})
			model.Bones = []*components.Bone{components.NewBone("root")}
			f.BindMdl(model)

			f.ExtractMDLLights()

			if len(model.Lights) != 1 {
				t.Fatalf("expected 1 light, got %d", len(model.Lights))
			}
			if got := model.Lights[0].LightType; got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestParticleNegativeDragPreservesAcceleration(t *testing.T) {
	for _, initial := range []float64{-10, 10} {
		speed := components.AnimatedOrStatic[float64]{Static: true, Value: initial}
		applyParticleDrag(&speed, 1.5, -1)
		if math.Abs(speed.Value) <= math.Abs(initial) || speed.Value*initial <= 0 {
			t.Fatalf("negative drag did not accelerate %v: %v", initial, speed.Value)
		}
		if got := calculateEquivalentVelocityNoDrag(initial, 1.5, 0); got != initial {
			t.Fatalf("zero drag changed velocity: %v", got)
		}
	}
}

func TestM2MotionBurstFlagDoesNotBecomeWC3Squirt(t *testing.T) {
	track := func(value float64) m2.Track {
		return m2.Track{GlobalSeq: config.BlizzardNull, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{value}}}}
	}
	f := &File{IsLoaded: true, m2Animations: []m2AnimMeta{{Duration: 2000}}, particleEmitters: []m2.ParticleEmitterEntry{{Flags: 0x20040, EmissionRate: track(25), Lifespan: track(2)}}}
	model := mdl.New(mdl.NewMDLOptions{Name: "motion burst"})
	model.Bones = []*components.Bone{components.NewBone("root")}
	f.BindMdl(model)
	f.ExtractMDLParticlesEmitters([]components.Texture{{Image: "smoke.blp"}})
	if len(model.ParticleEmitter2s) != 1 || model.ParticleEmitter2s[0].Squirt || model.ParticleEmitter2s[0].EmissionRate.Value != 25 {
		t.Fatal("M2 particles/second became WC3 emission-per-key")
	}
}

func TestParticleDragMatchesVisibleAge(t *testing.T) {
	p := m2.ParticleEmitterEntry{Drag: 2, AlphaTrack: m2.PartTrack{Timestamps: []uint16{0, 2171, 16384, 32767}, Values: [][]float64{{0}, {0}, {32767}, {0}}}}
	age := particleDragSampleAge(p, 2)
	if age < 1 || age > 1.1 {
		t.Fatalf("visible age %g, want near the one-second opacity peak", age)
	}
	visible := calculateEquivalentVelocityNoDrag(1, age, 2)
	endpoint := calculateEquivalentVelocityNoDrag(1, 2, 2)
	if visible < 1.5*endpoint {
		t.Fatal("visible cloud remains collapsed to its death-time travel speed")
	}
	p.Drag = -1
	if particleDragSampleAge(p, 2) != 2 {
		t.Fatal("negative-drag acceleration approximation changed")
	}
}
