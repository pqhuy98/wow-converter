package m2

import (
	"context"
	"encoding/binary"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

func TestParticleLoaderPreservesDirectionTracks(t *testing.T) {
	for _, field := range []string{"horizontal", "zSource"} {
		data := make([]byte, 560)
		put := func(at int, value uint32) { binary.LittleEndian.PutUint32(data[at:], value) }
		put(0, 1)
		put(4, 8)
		// Standard 476-byte particle record; horizontal range is its fourth M2Track.
		track := 8 + 52 + 3*20
		if field == "zSource" {
			track = 8 + 52 + 9*20 + 8
		}
		binary.LittleEndian.PutUint16(data[track+2:], 65535)
		put(track+4, 1)
		put(track+8, 520)
		put(track+12, 1)
		put(track+16, 528)
		put(520, 1)
		put(524, 536)
		put(528, 1)
		put(532, 540)
		put(536, 0)
		put(540, math.Float32bits(math.Pi))
		loader := NewLoader(buffer.From(data), nil)
		loader.parseMD21ParticleEmitters(0)
		got := loader.ParticleEmitters[0].HorizontalRange
		if field == "zSource" {
			got = loader.ParticleEmitters[0].ZSource
		}
		if got.GlobalSeq != 65535 || len(got.Values) != 1 || len(got.Values[0]) != 1 || math.Abs(got.Values[0][0][0]-math.Pi) > 1e-6 {
			t.Fatalf("%s discarded: %+v", field, got)
		}
	}
}

func TestEXP2OverridesLegacyParticleZSource(t *testing.T) {
	data := make([]byte, 8+8+28)
	put := func(at int, value uint32) { binary.LittleEndian.PutUint32(data[at:], value) }
	put(0, chunkEXP2)
	put(4, 36)
	put(8, 1)
	put(12, 8)
	put(16, math.Float32bits(0.25))
	loader := NewLoader(buffer.From(data), nil)
	loader.ParticleEmitters = []ParticleEmitterEntry{{ZSource: Track{Values: [][][]float64{{{255}}}}}}
	if err := loader.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := loader.ParticleEmitters[0].ZSource.Values[0][0][0]; got != 0.25 {
		t.Fatalf("EXP2 ignored: %v", got)
	}
	put(12, 1000)
	if err := NewLoader(buffer.From(data), nil).Load(context.Background()); err == nil {
		t.Fatal("accepted invalid EXP2 offsets")
	}
}

func TestParticleLoaderPreservesQuadSpin(t *testing.T) {
	data := make([]byte, 560)
	binary.LittleEndian.PutUint32(data, 1)
	binary.LittleEndian.PutUint32(data[4:], 8)
	values := [4]float32{.25, .5, 1, 2}
	for i, value := range values {
		binary.LittleEndian.PutUint32(data[8+376+i*4:], math.Float32bits(value))
	}
	loader := NewLoader(buffer.From(data), nil)
	loader.parseMD21ParticleEmitters(0)
	p := loader.ParticleEmitters[0]
	if got := [4]float32{p.BaseSpin, p.BaseSpinVary, p.Spin, p.SpinVary}; got != values {
		t.Fatalf("spin discarded or offset wrong: %v", got)
	}
}
