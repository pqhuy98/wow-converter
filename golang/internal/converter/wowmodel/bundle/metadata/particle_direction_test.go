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

func TestSphereParticleAngularBasis(t *testing.T) {
	track := func(v float64) m2.Track {
		return m2.Track{GlobalSeq: config.BlizzardNull, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{v}}}}
	}
	for _, test := range []struct {
		name                 string
		vertical, horizontal float64
		line                 bool
	}{
		{"cone", 0.3, 0.3, false}, {"circle", 0, math.Pi, true}, {"elevation", 0.3, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := &File{mdl: mdl.New(mdl.NewMDLOptions{Name: "angular basis"})}
			node := components.NewParticleEmitter2("sphere")
			f.orientSphereParticle(node, m2.ParticleEmitterEntry{EmitterType: 2, VerticalRange: track(test.vertical), HorizontalRange: track(test.horizontal)})
			line := false
			for _, flag := range node.Flags2 {
				line = line || flag == components.PE2LineEmitter
			}
			if line != test.line {
				t.Fatalf("LineEmitter=%v", line)
			}
			q := node.Rotation.KeyFrames[0].(imath.QuaternionRotation)
			azimuth, elevation := 0.23, -0.17
			var input, want imath.Vector3
			if test.name == "elevation" {
				input = imath.Vector3{0, math.Sin(elevation), math.Cos(elevation)}
				want = imath.Vector3{0, math.Cos(elevation), -math.Sin(elevation)}
			} else {
				if line {
					elevation = 0
				}
				input = imath.Vector3{math.Sin(elevation), math.Sin(azimuth) * math.Cos(elevation), math.Cos(azimuth) * math.Cos(elevation)}
				want = imath.Vector3{-math.Sin(azimuth) * math.Cos(elevation), math.Cos(azimuth) * math.Cos(elevation), -math.Sin(elevation)}
			}
			got := rotateParticleTest(q, input)
			for i := range got {
				if math.Abs(got[i]-want[i]) > 1e-8 {
					t.Fatalf("direction %v; native generator wants %v", got, want)
				}
			}
			if node.Rotation.GlobalSeq == nil || node.Rotation.GlobalSeq.Duration != 1 {
				t.Fatal("basis must persist across sequences")
			}
		})
	}
}

func TestSphereShellPreservesRateAndBindPoint(t *testing.T) {
	f := &File{mdl: mdl.New(mdl.NewMDLOptions{Name: "shell"})}
	track := func(v float64) m2.Track {
		return m2.Track{GlobalSeq: config.BlizzardNull, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{v}}}}
	}
	parent := components.NewBone("parent")
	node := components.NewParticleEmitter2("sphere")
	node.Parent = parent
	node.PivotPoint = imath.Vector3{1, 2, 3}
	node.Width = components.AnimatedOrStatic[float64]{Static: true, Value: 0.8}
	node.Length = node.Width
	node.EmissionRate = components.AnimatedOrStatic[float64]{Anim: &components.Animation{KeyFrames: map[int]any{0: float64(200), 1000: float64(50)}}}
	variants := f.particleDirectionVariants(node, m2.ParticleEmitterEntry{EmitterType: 2, HorizontalRange: track(math.Pi), VerticalRange: track(0)})
	if len(variants) != 32 {
		t.Fatalf("directions=%d", len(variants))
	}
	for _, key := range []int{0, 1000} {
		total := 0.0
		for _, variant := range variants {
			total += variant.EmissionRate.Anim.KeyFrames[key].(float64)
			if variant.Parent != parent || variant.PivotPoint != node.PivotPoint {
				t.Fatal("changed source bind point")
			}
			offset := variant.Translation.KeyFrames[0].(imath.Vector3)
			if math.Abs(math.Hypot(offset[0], offset[1])-0.8) > 1e-8 {
				t.Fatal("shell radius lost")
			}
		}
		if math.Abs(total-node.EmissionRate.Anim.KeyFrames[key].(float64)) > 1e-8 {
			t.Fatalf("total rate=%v", total)
		}
	}
	for _, particle := range []m2.ParticleEmitterEntry{{EmitterType: 1}, {EmitterType: 2, Flags: 0x100}} {
		plain := components.NewParticleEmitter2("unchanged")
		if got := f.particleDirectionVariants(plain, particle); len(got) != 1 || got[0].Rotation != nil {
			t.Fatal("plane/forced-up emitter changed")
		}
	}
}

func TestVerticalSphereCirclePreservesPortalHole(t *testing.T) {
	f := &File{mdl: mdl.New(mdl.NewMDLOptions{Name: "vertical portal"})}
	track := func(v float64) m2.Track {
		return m2.Track{GlobalSeq: config.BlizzardNull, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{v}}}}
	}
	node := components.NewParticleEmitter2("portal")
	node.Width = components.AnimatedOrStatic[float64]{Static: true, Value: 1.8}
	node.Length = node.Width
	node.Speed = components.AnimatedOrStatic[float64]{Static: true, Value: -1.1}
	node.LifeSpan = .5
	node.EmissionRate = components.AnimatedOrStatic[float64]{Static: true, Value: 500}
	variants := f.particleDirectionVariants(node, m2.ParticleEmitterEntry{
		EmitterType: 2, HorizontalRange: track(0), VerticalRange: track(math.Pi), EmissionRate: track(500),
	})
	if len(variants) != 16 {
		t.Fatalf("vertical circle fell back to a rectangle: %d sectors", len(variants))
	}
	total := 0.0
	for _, variant := range variants {
		total += variant.EmissionRate.Value
		offset := variant.Translation.KeyFrames[0].(imath.Vector3)
		direction := rotateParticleTest(variant.Rotation.KeyFrames[0].(imath.QuaternionRotation), imath.Vector3{0, 0, 1})
		if offset[0] != 0 || math.Abs(math.Hypot(offset[1], offset[2])-1.8) > 1e-8 {
			t.Fatalf("vertical radius lost: %v", offset)
		}
		for i := range offset {
			if math.Abs(direction[i]-offset[i]/1.8) > 1e-8 {
				t.Fatalf("inward velocity no longer follows birth radius: %v versus %v", direction, offset)
			}
		}
		if variant.Width.Value != 0 || variant.Length.Value > .8 || variant.Latitude.Value > 12 || variant.Speed.Value != -1.1 {
			t.Fatal("sector spread or source speed changed")
		}
	}
	if math.Abs(total-500) > 1e-8 {
		t.Fatal("portal emission budget changed")
	}
}

func rotateParticleTest(q imath.QuaternionRotation, v imath.Vector3) imath.Vector3 {
	u := imath.Vector3{q[0], q[1], q[2]}
	cross := imath.Vector3{u[1]*v[2] - u[2]*v[1], u[2]*v[0] - u[0]*v[2], u[0]*v[1] - u[1]*v[0]}
	dot := u[0]*v[0] + u[1]*v[1] + u[2]*v[2]
	norm := u[0]*u[0] + u[1]*u[1] + u[2]*u[2]
	return imath.Vector3{2*dot*u[0] + (q[3]*q[3]-norm)*v[0] + 2*q[3]*cross[0], 2*dot*u[1] + (q[3]*q[3]-norm)*v[1] + 2*q[3]*cross[1], 2*dot*u[2] + (q[3]*q[3]-norm)*v[2] + 2*q[3]*cross[2]}
}

func TestParticleDirectionTracksScaleOnce(t *testing.T) {
	model := mdl.New(mdl.NewMDLOptions{Name: "animated sectors"})
	f := &File{mdl: model}
	track := func(v float64) m2.Track {
		return m2.Track{GlobalSeq: config.BlizzardNull, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{v}}}}
	}
	node := components.NewParticleEmitter2("burst")
	node.LifeSpan = 0.5
	animated := func(v float64) components.AnimatedOrStatic[float64] {
		return components.AnimatedOrStatic[float64]{Anim: &components.Animation{KeyFrames: map[int]any{0: v}}}
	}
	node.Gravity, node.Speed = animated(2.5), animated(3)
	node.Width, node.Length = components.AnimatedOrStatic[float64]{Static: true}, components.AnimatedOrStatic[float64]{Static: true}
	node.EmissionRate = animated(150)
	model.ParticleEmitter2s = f.particleDirectionVariants(node, m2.ParticleEmitterEntry{EmitterType: 1, HorizontalRange: track(2 * math.Pi), VerticalRange: track(0.2), EmissionRate: track(150)})
	model.Modify.Scale(56)
	for _, variant := range model.ParticleEmitter2s {
		if variant.Gravity.Anim.KeyFrames[0] != float64(140) || variant.Speed.Anim.KeyFrames[0] != float64(168) {
			t.Fatal("sector tracks were scaled repeatedly")
		}
	}
	if node.Gravity.Anim.KeyFrames[0] != float64(2.5) {
		t.Fatal("source track mutated")
	}
}

func TestPlaneParticleZSourceOverridesAngularRanges(t *testing.T) {
	model := mdl.New(mdl.NewMDLOptions{Name: "source offset"})
	f := &File{mdl: model}
	track := func(v float64) m2.Track {
		return m2.Track{GlobalSeq: config.BlizzardNull, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{v}}}}
	}
	node := components.NewParticleEmitter2("fan")
	node.Gravity = components.AnimatedOrStatic[float64]{Static: true, Value: 3}
	variants := f.particleDirectionVariants(node, m2.ParticleEmitterEntry{EmitterType: 1, Flags: 0x10, ZSource: track(255), VerticalRange: track(math.Pi), HorizontalRange: track(2 * math.Pi), EmissionAreaWidth: track(1), EmissionAreaLength: track(1)})
	if len(variants) != 1 || node.Latitude.Value > 0.2 || node.Gravity.Value != -3 {
		t.Fatal("source offset or model-space gravity lost")
	}
	direction := rotateParticleTest(node.Rotation.KeyFrames[0].(imath.QuaternionRotation), imath.Vector3{0, 0, 1})
	if math.Abs(direction[2]+1) > 1e-6 {
		t.Fatalf("fan points %v", direction)
	}
}

func TestParticleBurstSectorClocksPreserveAgeAndRate(t *testing.T) {
	f := &File{mdl: mdl.New(mdl.NewMDLOptions{Name: "burst clocks"})}
	track := func(v float64) m2.Track {
		return m2.Track{GlobalSeq: config.BlizzardNull, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{v}}}}
	}
	particle := m2.ParticleEmitterEntry{EmitterType: 1, HorizontalRange: track(2 * math.Pi), VerticalRange: track(0.2), EmissionRate: track(150)}
	particle.EmissionRate.Values[0] = [][]float64{{150}, {0}}
	node := components.NewParticleEmitter2("burst")
	node.LifeSpan = 0.5
	node.Width, node.Length = components.AnimatedOrStatic[float64]{Static: true}, components.AnimatedOrStatic[float64]{Static: true}
	node.EmissionRate = components.AnimatedOrStatic[float64]{Static: true, Value: 150}
	variants := f.particleDirectionVariants(node, particle)
	if len(variants) > 9 {
		t.Fatal("burst clocks delay initial births")
	}
	total := 0.0
	for _, variant := range variants {
		total += variant.EmissionRate.Value
	}
	if math.Abs(total-150) > 1e-6 {
		t.Fatal("burst rate changed")
	}
}

func TestFullSphereModelSpaceDoesNotRotateGravity(t *testing.T) {
	track := func(v float64) m2.Track {
		return m2.Track{GlobalSeq: config.BlizzardNull, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{v}}}}
	}
	f := &File{mdl: mdl.New(mdl.NewMDLOptions{Name: "gravity"})}
	node := components.NewParticleEmitter2("sphere")
	node.Gravity = components.AnimatedOrStatic[float64]{Static: true, Value: 7}
	f.orientSphereParticle(node, m2.ParticleEmitterEntry{EmitterType: 2, Flags: 0x10, Gravity: track(7), HorizontalRange: track(math.Pi), VerticalRange: track(math.Pi)})
	if node.Rotation != nil {
		t.Fatal("full sphere rotation would turn vertical gravity sideways")
	}
	if node.Gravity.Value != 7 {
		t.Fatal("gravity was changed")
	}
}

func TestAnimatedSphereShellStaysCircularAndPreservesRadiusKeys(t *testing.T) {
	f := &File{mdl: mdl.New(mdl.NewMDLOptions{Name: "expanding ring"})}
	static := func(v float64) m2.Track {
		return m2.Track{GlobalSeq: config.BlizzardNull, Timestamps: [][]uint32{{0}}, Values: [][][]float64{{{v}}}}
	}
	node := components.NewParticleEmitter2("ring")
	node.LifeSpan = .5
	node.Width = components.AnimatedOrStatic[float64]{Anim: &components.Animation{Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: float64(.2), 600: float64(12)}}}
	node.Length = components.AnimatedOrStatic[float64]{Anim: &components.Animation{Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: float64(.4), 600: float64(12)}}}
	node.EmissionRate = components.AnimatedOrStatic[float64]{Static: true, Value: 500}
	variants := f.particleDirectionVariants(node, m2.ParticleEmitterEntry{EmitterType: 2, HorizontalRange: static(math.Pi), VerticalRange: static(0), EmissionRate: static(500)})
	if len(variants) != 32 {
		t.Fatalf("animated radius fell back to a rectangle: %d sectors", len(variants))
	}
	total := 0.0
	for _, v := range variants {
		total += v.EmissionRate.Value
		if !v.Width.Static || v.Width.Value != 0 || v.Length.Static || v.Length.Anim == nil {
			t.Fatal("radius was not replaced by an animated tangential arc")
		}
		for time, want := range map[int]float64{0: .3, 600: 12} {
			offset := v.Translation.KeyFrames[time].(imath.Vector3)
			if math.Abs(math.Hypot(offset[0], offset[1])-want) > 1e-6 || offset[2] != 0 {
				t.Fatalf("radius key %d lost: %v", time, offset)
			}
			dir := rotateParticleTest(v.Rotation.KeyFrames[0].(imath.QuaternionRotation), imath.Vector3{0, 0, 1})
			if math.Abs(dir[2]) > 1e-6 || dir[0]*offset[0]+dir[1]*offset[1] <= 0 {
				t.Fatal("ring velocity does not point outward in the ground plane")
			}
		}
	}
	if math.Abs(total-500) > 1e-6 {
		t.Fatal("ring birth budget changed")
	}
	f.mdl.ParticleEmitter2s = variants
	f.mdl.Modify.Scale(56)
	for _, v := range variants {
		offset := v.Translation.KeyFrames[600].(imath.Vector3)
		if math.Abs(math.Hypot(offset[0], offset[1])-672) > 1e-5 {
			t.Fatal("radius translation scaled more than once")
		}
	}
}
