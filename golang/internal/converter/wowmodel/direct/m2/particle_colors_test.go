package directm2

import (
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestParticleColorOverrideSlotsAndTrackOwnership(t *testing.T) {
	original := m2.PartTrack{Timestamps: []uint16{0, 15885, 32767}, Values: [][]float64{{41, 124, 255}, {94, 255, 255}, {41, 124, 255}}}
	emitters := []m2.ParticleEmitterEntry{{ParticleColorIndex: 11, ColorTrack: original}, {ParticleColorIndex: 12, ColorTrack: original}, {ParticleColorIndex: 13, ColorTrack: original}, {ColorTrack: original}}
	colors := &m2.ParticleColor{
		Start: [3]uint32{4294030226, 0xFF123456, 0xFFABCDEF},
		Mid:   [3]uint32{4294789662, 0xFF654321, 0xFFFEDCBA},
		End:   [3]uint32{4294259763, 0xFF010203, 0xFF040506},
	}
	applyParticleColors(emitters, colors)
	for slot := 0; slot < 3; slot++ {
		for key, packed := range [3]uint32{colors.Start[slot], colors.Mid[slot], colors.End[slot]} {
			want := []float64{float64((packed >> 16) & 255), float64((packed >> 8) & 255), float64(packed & 255)}
			if !reflect.DeepEqual(emitters[slot].ColorTrack.Values[key], want) {
				t.Fatalf("slot %d key %d: %v, want %v", slot, key, emitters[slot].ColorTrack.Values[key], want)
			}
		}
		if !reflect.DeepEqual(emitters[slot].ColorTrack.Timestamps, original.Timestamps) {
			t.Fatal("replacement changed source timing")
		}
	}
	if !reflect.DeepEqual(original.Values[0], []float64{41, 124, 255}) || !reflect.DeepEqual(emitters[3].ColorTrack, original) {
		t.Fatal("replacement mutated the source or an unindexed emitter")
	}
}
