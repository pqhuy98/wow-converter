package blp

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	pngwriter "github.com/pqhuy98/wow-converter/internal/formats/png"
)

func TestCompactBLP1AlphaPlanePreservesEveryMip(t *testing.T) {
	tests := []struct {
		name  string
		want  uint32
		alpha func(level, index int) byte
	}{
		{name: "opaque", want: 0, alpha: func(_, _ int) byte { return 255 }},
		{name: "binary", want: 1, alpha: func(_, i int) byte {
			if i%2 == 0 {
				return 0
			}
			return 255
		}},
		{name: "fully transparent", want: 1, alpha: func(_, _ int) byte { return 0 }},
		{name: "four bit", want: 4, alpha: func(_, i int) byte { return byte(i%16) * 17 }},
		{name: "eight bit", want: 8, alpha: func(_, i int) byte { return []byte{0, 64, 128, 255}[i%4] }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := makeTestBLP1(tt.alpha)
			compacted, err := compactBLP1AlphaPlane(original, false)
			if err != nil {
				t.Fatal(err)
			}
			if got := binary.LittleEndian.Uint32(compacted[blp1AlphaBitsOffset:]); got != tt.want {
				t.Fatalf("alphaBits = %d, want %d", got, tt.want)
			}
			if !bytes.Equal(original[blp1HeaderSize:blp1HeaderSize+blp1PaletteSize], compacted[blp1HeaderSize:blp1HeaderSize+blp1PaletteSize]) {
				t.Fatal("palette bytes changed")
			}
			for level := range 3 {
				oldOffset := int(binary.LittleEndian.Uint32(original[blp1MipOffsetTable+level*4:]))
				newOffset := int(binary.LittleEndian.Uint32(compacted[blp1MipOffsetTable+level*4:]))
				oldSize := int(binary.LittleEndian.Uint32(original[blp1MipSizeTable+level*4:]))
				if level == 0 && newOffset != blp1HeaderSize+blp1PaletteSize || level > 0 && newOffset != int(binary.LittleEndian.Uint32(compacted[blp1MipOffsetTable+(level-1)*4:]))+int(binary.LittleEndian.Uint32(compacted[blp1MipSizeTable+(level-1)*4:])) {
					t.Fatalf("mip %d offset was not updated to the compact layout: %d", level, newOffset)
				}
				if !bytes.Equal(original[oldOffset:oldOffset+oldSize/2], compacted[newOffset:newOffset+oldSize/2]) {
					t.Fatalf("mip %d color index bytes changed", level)
				}
				before := decodeTestBLP1Mip(t, original, level)
				after := decodeTestBLP1Mip(t, compacted, level)
				if len(before) != len(after) {
					t.Fatalf("mip %d decoded length %d, want %d", level, len(after), len(before))
				}
				for i := range before {
					if before[i] != after[i] {
						t.Fatalf("mip %d pixel %d changed: %v -> %v", level, i, before[i], after[i])
					}
				}
				pixels := len(before)
				newSize := binary.LittleEndian.Uint32(compacted[blp1MipSizeTable+level*4:])
				if tt.want != 8 {
					planeBytes := 0
					switch tt.want {
					case 0:
					case 1:
						planeBytes = (pixels + 7) / 8
					case 4:
						planeBytes = (pixels + 1) / 2
					}
					if int(newSize) != pixels+planeBytes {
						t.Fatalf("mip %d payload size = %d, want %d", level, newSize, pixels+planeBytes)
					}
				}
			}
		})
	}
}

func TestCompactBLP1AlphaPlaneRejectsMalformedPalettedInput(t *testing.T) {
	valid := makeTestBLP1(func(_, _ int) byte { return 128 })
	tests := []struct {
		name string
		data []byte
	}{
		{name: "short header", data: []byte("BLP1\x01")},
		{name: "short palette", data: valid[:blp1HeaderSize+20]},
	}
	badOffset := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(badOffset[blp1MipOffsetTable:], uint32(len(badOffset)+1))
	tests = append(tests, struct {
		name string
		data []byte
	}{name: "out of bounds mip", data: badOffset})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := compactBLP1AlphaPlane(tt.data, false); err == nil {
				t.Fatal("expected malformed paletted BLP1 to be rejected")
			}
		})
	}
}

func TestIgnoreAlphaDropsOnlyPlaneAndPreservesRGB(t *testing.T) {
	original := makeTestBLP1(func(_, i int) byte { return []byte{0, 64, 128, 255}[i%4] })
	compacted, err := compactBLP1AlphaPlane(original, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(compacted[blp1AlphaBitsOffset:]); got != 0 {
		t.Fatalf("alphaBits = %d, want 0", got)
	}
	for level := range 3 {
		before := decodeTestBLP1Mip(t, original, level)
		after := decodeTestBLP1Mip(t, compacted, level)
		for i := range before {
			if before[i].R != after[i].R || before[i].G != after[i].G || before[i].B != after[i].B {
				t.Fatalf("mip %d pixel %d RGB changed: %v -> %v", level, i, before[i], after[i])
			}
			if after[i].A != 255 {
				t.Fatalf("mip %d pixel %d alpha = %d, want opaque after alpha discard", level, i, after[i].A)
			}
		}
	}
}

func TestPng2BlpJSWritesCompactedBLP1(t *testing.T) {
	rgba := []byte{10, 20, 30, 255, 40, 50, 60, 255, 70, 80, 90, 255, 100, 110, 120, 255}
	pngBytes, err := pngwriter.EncodeRGBA(rgba, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "opaque.blp")
	if err := Png2BlpJS(pngBytes, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(data[blp1AlphaBitsOffset:]); got != 0 {
		t.Fatalf("fallback alphaBits = %d, want opaque 0-bit plane", got)
	}
}

func TestPng2BlpJSIgnoreAlphaDoesNotChangeQuantization(t *testing.T) {
	rgba := make([]byte, 4*4*4)
	colors := [][4]byte{{255, 0, 0, 10}, {0, 255, 0, 70}, {0, 0, 255, 130}, {255, 255, 0, 190}}
	for i := 0; i < 16; i++ {
		copy(rgba[i*4:i*4+4], colors[i%len(colors)][:])
	}
	pngBytes, err := pngwriter.EncodeRGBA(rgba, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	regularPath := filepath.Join(dir, "regular.blp")
	ignoredPath := filepath.Join(dir, "ignored.blp")
	if err := png2BlpJS(pngBytes, regularPath, false); err != nil {
		t.Fatal(err)
	}
	if err := png2BlpJS(pngBytes, ignoredPath, true); err != nil {
		t.Fatal(err)
	}
	regular, err := os.ReadFile(regularPath)
	if err != nil {
		t.Fatal(err)
	}
	ignored, err := os.ReadFile(ignoredPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(ignored[blp1AlphaBitsOffset:]); got != 0 {
		t.Fatalf("ignored alphaBits = %d, want 0", got)
	}
	if !bytes.Equal(regular[blp1HeaderSize:blp1HeaderSize+blp1PaletteSize], ignored[blp1HeaderSize:blp1HeaderSize+blp1PaletteSize]) {
		t.Fatal("ignoring alpha changed fallback palette quantization")
	}
	regularOffset := int(binary.LittleEndian.Uint32(regular[blp1MipOffsetTable:]))
	ignoredOffset := int(binary.LittleEndian.Uint32(ignored[blp1MipOffsetTable:]))
	if !bytes.Equal(regular[regularOffset:regularOffset+16], ignored[ignoredOffset:ignoredOffset+16]) {
		t.Fatal("ignoring alpha changed fallback color indices")
	}
}

func TestAlphaDepthReaderSupportsCompactedBitDepths(t *testing.T) {
	tests := []struct {
		name  string
		depth uint8
		raw   []byte
		want  []uint8
	}{
		{name: "zero", depth: 0, want: []uint8{255}},
		{name: "one", depth: 1, raw: []byte{0, 0, 0, 0b00000101}, want: []uint8{255, 0, 255}},
		{name: "four", depth: 4, raw: []byte{0, 0, 0, 0, 0x10, 0xF2}, want: []uint8{0, 17, 34, 255}},
		{name: "eight", depth: 8, raw: []byte{0, 0, 0, 0, 0, 64, 128, 255}, want: []uint8{0, 64, 128, 255}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := &BLPImage{AlphaDepth: tt.depth, scaledLength: len(tt.want), rawData: tt.raw}
			for i, want := range tt.want {
				if got := img.getAlpha(i); got != want {
					t.Fatalf("alpha[%d] = %d, want %d", i, got, want)
				}
			}
		})
	}
}

func makeTestBLP1(alpha func(level, index int) byte) []byte {
	dims := []int{4, 2, 1}
	paletteEnd := blp1HeaderSize + blp1PaletteSize
	out := make([]byte, paletteEnd)
	copy(out[:4], "BLP1")
	binary.LittleEndian.PutUint32(out[4:8], 1)
	binary.LittleEndian.PutUint32(out[blp1AlphaBitsOffset:], 8)
	binary.LittleEndian.PutUint32(out[blp1WidthOffset:], 4)
	binary.LittleEndian.PutUint32(out[blp1HeightOffset:], 4)
	for i := range 256 {
		p := blp1HeaderSize + i*4
		out[p], out[p+1], out[p+2], out[p+3] = byte(i), byte(255-i), byte(i/2), 255
	}
	for level, dim := range dims {
		pixels := dim * dim
		offset := len(out)
		mip := make([]byte, pixels*2)
		for i := range pixels {
			mip[i] = byte((i*37 + level*53) % 256)
			mip[pixels+i] = alpha(level, i)
		}
		out = append(out, mip...)
		binary.LittleEndian.PutUint32(out[blp1MipOffsetTable+level*4:], uint32(offset))
		binary.LittleEndian.PutUint32(out[blp1MipSizeTable+level*4:], uint32(len(mip)))
	}
	return out
}

func decodeTestBLP1Mip(t *testing.T, data []byte, level int) []color.NRGBA {
	t.Helper()
	width, height := int(binary.LittleEndian.Uint32(data[blp1WidthOffset:])), int(binary.LittleEndian.Uint32(data[blp1HeightOffset:]))
	width, height = max(1, width>>level), max(1, height>>level)
	pixels := width * height
	offset := int(binary.LittleEndian.Uint32(data[blp1MipOffsetTable+level*4:]))
	alphaBits := binary.LittleEndian.Uint32(data[blp1AlphaBitsOffset:])
	indices := data[offset : offset+pixels]
	alphaOffset := offset + pixels
	decoded := make([]color.NRGBA, pixels)
	for i, index := range indices {
		p := blp1HeaderSize + int(index)*4
		a := byte(255)
		switch alphaBits {
		case 1:
			a = 0
			if data[alphaOffset+i/8]&(1<<uint(i%8)) != 0 {
				a = 255
			}
		case 4:
			nibble := data[alphaOffset+i/2] >> uint((i%2)*4) & 0x0F
			a = nibble * 17
		case 8:
			a = data[alphaOffset+i]
		}
		decoded[i] = color.NRGBA{R: data[p+2], G: data[p+1], B: data[p], A: a}
	}
	return decoded
}
