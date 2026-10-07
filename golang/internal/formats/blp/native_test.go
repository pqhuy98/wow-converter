package blp

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	stdpng "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/blp/cnative"
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

func TestConvertPngToBlpFallsBackWhenNativeRejectsValidSmallPNG(t *testing.T) {
	if !NativeEncoderAvailable() {
		t.Skip("native BLP encoder unavailable (run scripts/build-blp-native.ps1 or .sh)")
	}

	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 19, G: 211, B: 87, A: 255})
	var pngBytes bytes.Buffer
	if err := stdpng.Encode(&pngBytes, img); err != nil {
		t.Fatalf("encode standard-library PNG: %v", err)
	}
	if _, err := stdpng.Decode(bytes.NewReader(pngBytes.Bytes())); err != nil {
		t.Fatalf("standard-library PNG is invalid: %v", err)
	}
	if _, err := cnative.EncodePng(pngBytes.Bytes()); err == nil {
		t.Skip("native encoder accepts this PNG; no fallback needed on this build")
	}

	outPath := filepath.Join(t.TempDir(), "small.blp")
	if err := ConvertPngToBlp(pngBytes.Bytes(), outPath); err != nil {
		t.Fatalf("ConvertPngToBlp after native rejection: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read fallback BLP: %v", err)
	}
	if len(data) < 4 || string(data[:4]) != "BLP1" {
		t.Fatalf("unexpected fallback BLP header: %q", data[:min(8, len(data))])
	}

	blockedParent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedParent, []byte("file"), 0o644); err != nil {
		t.Fatalf("create blocked parent: %v", err)
	}
	err = ConvertPngToBlp(pngBytes.Bytes(), filepath.Join(blockedParent, "failure.blp"))
	if err == nil || !strings.Contains(err.Error(), "native encoder:") || !strings.Contains(err.Error(), "Go fallback:") {
		t.Fatalf("want both native and fallback errors, got %v", err)
	}
}
