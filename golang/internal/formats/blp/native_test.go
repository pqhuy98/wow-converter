package blp

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	pngwriter "github.com/pqhuy98/wow-converter/internal/formats/png"
)

func TestNativeEncoderRoundTrip(t *testing.T) {
	if !NativeEncoderAvailable() {
		t.Skip("native BLP encoder unavailable (run scripts/build-blp-native.ps1 or .sh)")
	}

	rgba := make([]byte, 4*4*4)
	for i := 0; i < len(rgba); i += 4 {
		rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = 255, 0, 0, 255
	}
	pngBytes, err := pngwriter.EncodeRGBA(rgba, 4, 4)
	if err != nil {
		t.Fatalf("encode png: %v", err)
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "test.blp")
	if err := ConvertPngToBlp(pngBytes, outPath); err != nil {
		t.Fatalf("ConvertPngToBlp: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read blp: %v", err)
	}
	if len(data) < 20 || string(data[:4]) != "BLP1" {
		t.Fatalf("unexpected blp header: %q", data[:min(8, len(data))])
	}
	if bits := binary.LittleEndian.Uint32(data[blp1AlphaBitsOffset:]); bits != 0 {
		t.Fatalf("opaque native output alphaBits = %d, want compact 0-bit plane (compression=%d dimensions=%dx%d)", bits, binary.LittleEndian.Uint32(data[4:8]), binary.LittleEndian.Uint32(data[12:16]), binary.LittleEndian.Uint32(data[16:20]))
	}
}
