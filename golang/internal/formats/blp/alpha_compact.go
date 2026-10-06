package blp

import (
	"encoding/binary"
	"fmt"
)

const (
	blp1AlphaBitsOffset = 8
	blp1WidthOffset     = 12
	blp1HeightOffset    = 16
	blp1MipOffsetTable  = 28
	blp1MipSizeTable    = 92
	blp1MipSlots        = 16
	blp1PaletteSize     = 256 * 4
)

type blp1Mip struct {
	offset int
	size   int
	pixels int
	alpha  []byte
}

// compactBLP1AlphaPlane reduces only paletted BLP1's separate 8-bit alpha
// planes. Palette and index bytes are copied verbatim.
func compactBLP1AlphaPlane(data []byte, ignoreAlpha bool) ([]byte, error) {
	if len(data) < 4 || string(data[:4]) != "BLP1" {
		return data, nil
	}
	if len(data) < blp1HeaderSize {
		return nil, fmt.Errorf("truncated BLP1 header: %d bytes", len(data))
	}
	if binary.LittleEndian.Uint32(data[4:8]) != 1 {
		return data, nil
	}
	inputAlphaBits := binary.LittleEndian.Uint32(data[blp1AlphaBitsOffset : blp1AlphaBitsOffset+4])
	if inputAlphaBits == 0 && ignoreAlpha {
		return data, nil
	}
	if !ignoreAlpha && inputAlphaBits != 8 {
		return data, nil
	}
	if inputAlphaBits != 1 && inputAlphaBits != 4 && inputAlphaBits != 8 {
		return nil, fmt.Errorf("unsupported paletted BLP1 alpha depth %d", inputAlphaBits)
	}
	paletteEnd := blp1HeaderSize + blp1PaletteSize
	if len(data) < paletteEnd {
		return nil, fmt.Errorf("truncated paletted BLP1 palette: %d bytes", len(data))
	}
	width := binary.LittleEndian.Uint32(data[blp1WidthOffset : blp1WidthOffset+4])
	height := binary.LittleEndian.Uint32(data[blp1HeightOffset : blp1HeightOffset+4])
	if width == 0 || height == 0 || uint64(width)*uint64(height) > uint64(len(data)) {
		return nil, fmt.Errorf("invalid paletted BLP1 dimensions %dx%d", width, height)
	}

	mips := make([]blp1Mip, 0, blp1MipSlots)
	var opaque, binaryAlpha, alpha4Bit = true, true, true
	lastEnd := paletteEnd
	terminated := false
	ceilW, ceilH := int(width), int(height)
	floorW, floorH := int(width), int(height)
	ceilPlan, floorPlan := true, true
	for level := 0; level < blp1MipSlots; level++ {
		offset := binary.LittleEndian.Uint32(data[blp1MipOffsetTable+level*4 : blp1MipOffsetTable+level*4+4])
		size := binary.LittleEndian.Uint32(data[blp1MipSizeTable+level*4 : blp1MipSizeTable+level*4+4])
		if offset == 0 && size == 0 {
			terminated = true
			continue
		}
		if terminated || offset == 0 || size == 0 {
			return nil, fmt.Errorf("invalid BLP1 mip table entry %d", level)
		}
		if uint64(offset)+uint64(size) > uint64(len(data)) || int(offset) < lastEnd {
			return nil, fmt.Errorf("invalid BLP1 mip range %d: offset=%d size=%d", level, offset, size)
		}
		ceilPixels, floorPixels := ceilW*ceilH, floorW*floorH
		matchesCeil := ceilPlan && size == uint32(ceilPixels+blp1AlphaPlaneBytes(ceilPixels, inputAlphaBits))
		matchesFloor := floorPlan && size == uint32(floorPixels+blp1AlphaPlaneBytes(floorPixels, inputAlphaBits))
		if !matchesCeil && !matchesFloor {
			return nil, fmt.Errorf("invalid BLP1 mip dimensions at level %d", level)
		}
		ceilPlan = matchesCeil
		floorPlan = matchesFloor
		pixels := floorPixels
		if matchesCeil {
			pixels = ceilPixels
		}
		mipOffset, mipSize := int(offset), int(size)
		var alpha []byte
		if inputAlphaBits == 8 {
			alphaStart := mipOffset + pixels
			alpha = data[alphaStart : alphaStart+pixels]
			for _, a := range alpha {
				if a != 255 {
					opaque = false
				}
				if a != 0 && a != 255 {
					binaryAlpha = false
				}
				if a%17 != 0 {
					alpha4Bit = false
				}
			}
		}
		mips = append(mips, blp1Mip{offset: mipOffset, size: mipSize, pixels: pixels, alpha: alpha})
		lastEnd = mipOffset + mipSize
		ceilW, ceilH = max(1, (ceilW+1)/2), max(1, (ceilH+1)/2)
		floorW, floorH = max(1, floorW/2), max(1, floorH/2)
	}
	if len(mips) == 0 {
		return nil, fmt.Errorf("paletted BLP1 has no mip levels")
	}

	alphaBits := uint32(8)
	if ignoreAlpha {
		alphaBits = 0
	} else {
		switch {
		case opaque:
			alphaBits = 0
		case binaryAlpha:
			alphaBits = 1
		case alpha4Bit:
			alphaBits = 4
		}
	}
	if alphaBits == 8 {
		return data, nil
	}

	out := make([]byte, 0, len(data))
	out = append(out, data[:paletteEnd]...)
	binary.LittleEndian.PutUint32(out[blp1AlphaBitsOffset:blp1AlphaBitsOffset+4], alphaBits)
	cursor := paletteEnd
	for level, mip := range mips {
		out = append(out, data[cursor:mip.offset]...)
		newOffset := len(out)
		newAlpha := packBLP1Alpha(mip.alpha, alphaBits)
		out = append(out, data[mip.offset:mip.offset+mip.pixels]...)
		out = append(out, newAlpha...)
		newSize := mip.pixels + len(newAlpha)
		binary.LittleEndian.PutUint32(out[blp1MipOffsetTable+level*4:blp1MipOffsetTable+level*4+4], uint32(newOffset))
		binary.LittleEndian.PutUint32(out[blp1MipSizeTable+level*4:blp1MipSizeTable+level*4+4], uint32(newSize))
		cursor = mip.offset + mip.size
	}
	out = append(out, data[cursor:]...)
	return out, nil
}

func packBLP1Alpha(alpha []byte, bits uint32) []byte {
	switch bits {
	case 0:
		return nil
	case 1:
		packed := make([]byte, (len(alpha)+7)/8)
		for i, a := range alpha {
			if a == 255 {
				packed[i/8] |= 1 << (i % 8)
			}
		}
		return packed
	case 4:
		packed := make([]byte, (len(alpha)+1)/2)
		for i, a := range alpha {
			nibble := a / 17
			if i%2 == 0 {
				packed[i/2] = nibble
			} else {
				packed[i/2] |= nibble << 4
			}
		}
		return packed
	default:
		return append([]byte(nil), alpha...)
	}
}

func blp1AlphaPlaneBytes(pixels int, bits uint32) int {
	switch bits {
	case 1:
		return (pixels + 7) / 8
	case 4:
		return (pixels + 1) / 2
	case 8:
		return pixels
	default:
		return 0
	}
}
