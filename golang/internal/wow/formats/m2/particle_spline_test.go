package m2

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

func TestParticleSplineControlPoints(t *testing.T) {
	for _, offset := range []uint32{500, 900} {
		data := make([]byte, 560)
		put := func(at int, value uint32) { binary.LittleEndian.PutUint32(data[at:], value) }
		put(0, 1)
		put(4, 8)
		// Spline array header follows the four follow-speed/scale fields.
		put(8+448, 2)
		put(8+452, offset)
		for i, value := range []float32{1, 2, 3, 4, 5, 6} {
			put(500+i*4, math.Float32bits(value))
		}
		loader := NewLoader(buffer.From(data), nil)
		loader.parseMD21ParticleEmitters(0)
		points := loader.ParticleEmitters[0].SplinePoints
		if offset == 900 {
			if len(points) != 0 {
				t.Fatal("read out-of-bounds spline")
			}
		} else if len(points) != 2 || points[0] != [3]float32{1, 2, 3} || points[1] != [3]float32{4, 5, 6} {
			t.Fatalf("native spline coordinates lost: %v", points)
		}
	}
}
