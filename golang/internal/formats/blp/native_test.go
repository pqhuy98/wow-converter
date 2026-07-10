package blp

import (
	"os"
	"path/filepath"
	"testing"

	pngwriter "github.com/pqhuy98/wow-converter/internal/formats/png"
)

func TestNativeEncoderRoundTrip(t *testing.T) {
	if !NativeEncoderAvailable() {
		t.Skip("native BLP encoder unavailable (run scripts/build-blp-native.ps1 or .sh)")
	}

	pngBytes, err := pngwriter.EncodeRGBA([]byte{255, 0, 0, 255}, 4, 4)
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
}
