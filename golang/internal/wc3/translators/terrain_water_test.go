package translators

import (
	"bytes"
	"encoding/binary"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"math"
	"testing"
)

func TestWaterNativeEncodingAndBoundary(t *testing.T) {
	// Independently specified one-corner W3E, with a water height of 100 Z,
	// a boundary bit, and water presence. v12 moves presence to bit 0x100.
	fixture := func(version uint32) []byte {
		b := new(bytes.Buffer)
		b.WriteString("W3E!")
		put := func(v uint32) { _ = binary.Write(b, binary.LittleEndian, v) }
		put(version)
		b.WriteByte('L')
		put(1)
		put(0)
		put(0)
		put(1)
		put(1)
		put(0)
		put(0)
		_ = binary.Write(b, binary.LittleEndian, uint16(8192))
		_ = binary.Write(b, binary.LittleEndian, uint16(8950|0x4000))
		if version == 12 {
			_ = binary.Write(b, binary.LittleEndian, uint16(0x100))
		} else {
			b.WriteByte(0x40)
		}
		b.WriteByte(0)
		b.WriteByte(7)
		return b.Bytes()
	}
	for _, version := range []uint32{11, 12} {
		parsed := TerrainTranslator{}.WarToJSON(fixture(version)).JSON
		if parsed.WaterHeight[0][0] != 8950 || !parsed.BoundaryFlag[0][0] || parsed.Flags[0][0] != uint16(data.TerrainWater) {
			t.Fatalf("v%d lost native water fields: %+v", version, parsed)
		}
		if got := (TerrainTranslator{}).JSONToWar(parsed).Buffer; !bytes.Equal(got, fixture(12)) {
			t.Fatalf("v%d did not normalize to native v12 bytes", version)
		}
	}
	if config.GameZToWaterHeight(100) != 8950 {
		t.Fatal("missing factor four in water height encoder")
	}
	for _, z := range []float64{-2000, -89.6, 0, 100, 1900} {
		if d := math.Abs(config.WaterHeightToGameZ(float64(config.GameZToWaterHeight(z))) - z); d > .126 {
			t.Fatalf("height %.4f loses precision: %g", z, d)
		}
	}
}
