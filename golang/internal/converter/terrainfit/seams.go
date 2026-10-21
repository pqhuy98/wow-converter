package terrainfit

import (
	"image"
	"math"
)

type seamScore struct {
	Pixels        int64   `json:"windows"`
	MAE           float64 `json:"mae"`
	P95           int     `json:"p95"`
	ReferenceStep float64 `json:"referenceStep"`
	ResultStep    float64 `json:"resultStep"`
}

type seamMetrics struct {
	Native      seamScore `json:"native"`
	NativeModel seamScore `json:"nativeModel"`
	All         seamScore `json:"all"`
}

type seamSum struct {
	pixels                 int64
	error, reference, step float64
	histogram              [511]int64
}

type seamMeasurement struct{ native, mixed, all seamSum }

func (m *seamMeasurement) add(ref, result *image.NRGBA, mask, coverage []bool) {
	jump := func(img *image.NRGBA, at, step int) [3]float64 {
		var out [3]float64
		for k := range out {
			out[k] = 1.5*(float64(img.Pix[at+k])-float64(img.Pix[at-step+k])) - .5*(float64(img.Pix[at+step+k])-float64(img.Pix[at-2*step+k]))
		}
		return out
	}
	add := func(at, step, ca, cb int) {
		if !coverage[ca] || !coverage[cb] {
			return
		}
		a, b := jump(ref, at, step), jump(result, at, step)
		m.all.add(a, b)
		if !mask[ca] && !mask[cb] {
			m.native.add(a, b)
		} else if mask[ca] != mask[cb] {
			m.mixed.add(a, b)
		}
	}
	const size, cell = 1024, 1024 / cellsPerADT
	for y := 0; y < size; y++ {
		for x := cell; x < size; x += cell {
			b := y*ref.Stride + x*4
			cb := y/cell*cellsPerADT + x/cell
			add(b, 4, cb-1, cb)
		}
	}
	for y := cell; y < size; y += cell {
		for x := 0; x < size; x++ {
			b := y*ref.Stride + x*4
			cb := y/cell*cellsPerADT + x/cell
			add(b, ref.Stride, cb-cellsPerADT, cb)
		}
	}
}

func (m *seamSum) add(reference, result [3]float64) {
	m.pixels++
	for k := 0; k < 3; k++ {
		m.error += math.Abs(result[k] - reference[k])
		m.reference += math.Abs(reference[k])
		m.step += math.Abs(result[k])
		m.histogram[min(510, int(math.Abs(result[k])+.5))]++
	}
}

func (m seamSum) metrics() seamScore {
	if m.pixels == 0 {
		return seamScore{}
	}
	count, cumulative, p95 := m.pixels*3, int64(0), 0
	for i, n := range m.histogram {
		cumulative += n
		if cumulative >= int64(math.Ceil(float64(count)*.95)) {
			p95 = i
			break
		}
	}
	return seamScore{Pixels: m.pixels, MAE: m.error / float64(count), P95: p95, ReferenceStep: m.reference / float64(count), ResultStep: m.step / float64(count)}
}

func (m seamMeasurement) metrics() seamMetrics {
	return seamMetrics{Native: m.native.metrics(), NativeModel: m.mixed.metrics(), All: m.all.metrics()}
}
