package png

import (
	"bytes"
	"testing"
)

func TestResizePngBytesPreservesAlphaIndependently(t *testing.T) {
	// 2x2 RGBA: opaque red + transparent green mask pattern
	src := []byte{
		255, 0, 0, 255, 0, 255, 0, 0,
		0, 0, 255, 128, 255, 255, 255, 0,
	}
	pngBytes, err := EncodeRGBA(src, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ResizePngBytes(pngBytes, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	data, w, h, err := DecodeRGBA(out)
	if err != nil {
		t.Fatal(err)
	}
	if w != 4 || h != 4 {
		t.Fatalf("expected 4x4, got %dx%d", w, h)
	}
	if len(data) != 4*4*4 {
		t.Fatalf("unexpected output length %d", len(data))
	}
}

func TestIsAbnormalTransparency(t *testing.T) {
	src := make([]byte, 4*2*4)
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			i := (y*4 + x) * 4
			src[i+3] = 255
			if x%2 == 1 {
				src[i+3] = 0
			}
		}
	}
	pngBytes, err := EncodeRGBA(src, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := IsAbnormalTransparency(pngBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected abnormal transparency")
	}
}

func TestRemoveAlphaRGB(t *testing.T) {
	data := []byte{10, 20, 30, 0, 40, 50, 60, 255}
	RemoveAlphaRGB(data)
	if data[0] != 0 || data[1] != 0 || data[2] != 0 {
		t.Fatalf("expected transparent RGB cleared, got %v", data[:4])
	}
	if data[4] != 40 {
		t.Fatalf("expected opaque pixel preserved")
	}
}

func TestResizePngFill(t *testing.T) {
	src := bytes.Repeat([]byte{1, 2, 3, 4}, 1)
	pngBytes, err := EncodeRGBA(append([]byte(nil), src...), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ResizePngFill(pngBytes, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	_, w, h, err := DecodeRGBA(out)
	if err != nil || w != 2 || h != 2 {
		t.Fatalf("resize fill failed: %v %dx%d", err, w, h)
	}
}
