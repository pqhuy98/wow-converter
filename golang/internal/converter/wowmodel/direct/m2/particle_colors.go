package directm2

import (
	"math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func applyParticleColors(emitters []m2.ParticleEmitterEntry, colors *m2.ParticleColor) {
	if colors == nil {
		return
	}
	for i := range emitters {
		p := &emitters[i]
		if p.ParticleColorIndex < 11 || p.ParticleColorIndex > 13 || len(p.ColorTrack.Values) != 3 {
			continue
		}
		slot := p.ParticleColorIndex - 11
		p.ColorTrack.Values = make([][]float64, 3)
		for key, packed := range [3]uint32{colors.Start[slot], colors.Mid[slot], colors.End[slot]} {
			p.ColorTrack.Values[key] = []float64{float64((packed >> 16) & 255), float64((packed >> 8) & 255), float64(packed & 255)}
		}
	}
}

func applyParticleDisplayScale(emitters []m2.ParticleEmitterEntry, displayScale float64) {
	if displayScale <= 0 || displayScale == 1 || math.IsNaN(displayScale) || math.IsInf(displayScale, 0) {
		return
	}
	for i := range emitters {
		p := &emitters[i]
		if p.Flags&0x20 != 0 {
			continue
		}
		// Mesh units stay unscaled in the export. WoW only multiplies particle
		// size by the display scale when 0x20 is set, so preserve the ratio.
		values := make([][]float64, len(p.ScaleTrack.Values))
		for key, value := range p.ScaleTrack.Values {
			values[key] = append([]float64(nil), value...)
			for axis := range values[key] {
				values[key][axis] /= displayScale
			}
		}
		p.ScaleTrack.Values = values
	}
}
