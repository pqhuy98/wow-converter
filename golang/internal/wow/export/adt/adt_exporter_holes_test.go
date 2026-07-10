package adt

import (
	"testing"

	adtfmt "github.com/pqhuy98/wow-converter/internal/wow/formats/adt"
)

func holeGridPositions() map[int][2]int {
	out := make(map[int][2]int)
	xx, yy := 0, 0
	for j := 9; j < 145; j++ {
		if xx >= 8 {
			xx = 0
			yy++
		}
		out[j] = [2]int{xx, yy}
		xx++
		if (j+1)%(9+8) == 0 {
			j += 9
		}
	}
	return out
}

func legacyHoleGridPositions() map[int][2]int {
	out := make(map[int][2]int)
	for j := 9; j < 145; j++ {
		out[j] = [2]int{(j - 9) % 8, (j - 9) / 8}
		if (j+1)%(9+8) == 0 {
			j += 9
		}
	}
	return out
}

func TestHoleGridMatchesTypeScriptAfterRowSkips(t *testing.T) {
	current := holeGridPositions()
	legacy := legacyHoleGridPositions()

	mismatches := 0
	maxLegacyYY := 0
	for j, xy := range current {
		old := legacy[j]
		if old != xy {
			mismatches++
		}
		if old[1] > maxLegacyYY {
			maxLegacyYY = old[1]
		}
	}

	if mismatches == 0 {
		t.Fatal("expected legacy xx/yy formula to diverge after row skips")
	}
	if maxLegacyYY < 8 {
		t.Fatalf("expected legacy yy to exceed high-res hole table length, got max yy=%d", maxLegacyYY)
	}

	if xy, ok := current[26]; !ok || xy != [2]int{0, 1} {
		t.Fatalf("j=26 should map to xx=0 yy=1 (TS), got %v", xy)
	}
	if xy, ok := legacy[26]; !ok || xy != [2]int{1, 2} {
		t.Fatalf("legacy j=26 maps to xx=1 yy=2, got %v", xy)
	}
}

func TestHighResHoleOutOfBoundsRendersTriangle(t *testing.T) {
	chunk := adtfmt.ADTChunk{
		Flags:        0x10000,
		HolesHighRes: make([]uint8, 8),
	}

	if isChunkHole(chunk, true, 0, 8) {
		t.Fatal("yy=8 is out of bounds and should render (not a hole)")
	}
	if isChunkHole(chunk, true, 0, 0) {
		t.Fatal("cleared high-res bit should render")
	}
	chunk.HolesHighRes[0] = 0x01
	if !isChunkHole(chunk, true, 0, 0) {
		t.Fatal("set high-res bit should skip triangle")
	}
}

func isChunkHole(chunk adtfmt.ADTChunk, includeHoles bool, xx, yy int) bool {
	isHole := true
	if includeHoles {
		if chunk.Flags&0x10000 == 0 {
			current := 1 << (xx/2 + (yy/2)*4)
			if chunk.HolesLowRes&uint16(current) == 0 {
				isHole = false
			}
		} else {
			var hr uint8
			if yy < len(chunk.HolesHighRes) {
				hr = chunk.HolesHighRes[yy]
			}
			if (hr>>xx)&1 == 0 {
				isHole = false
			}
		}
	} else {
		isHole = false
	}
	return isHole
}
