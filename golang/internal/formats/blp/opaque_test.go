package blp

import (
	"os"
	"path/filepath"
	"testing"

	pngwriter "github.com/pqhuy98/wow-converter/internal/formats/png"
)

func TestOpaqueEncodeKeepsZeroAlphaRGB(t *testing.T) {
	const w, h = 20, 20
	pix := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		pix[i*4] = byte(80 + (i % 120))
		pix[i*4+1] = byte(i / 3)
		pix[i*4+2] = byte((i * 5) % 200)
	}
	pix[0], pix[1], pix[2], pix[3] = 0, 255, 0, 255
	pngBytes, err := pngwriter.EncodeRGBA(pix, w, h)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()

	dropped := filepath.Join(dir, "dropped.blp")
	if err := ConvertTextureToBlp(EncodeInput{PNG: pngBytes}, dropped); err != nil {
		t.Fatal(err)
	}
	if r, g, _ := meanBLP1(t, dropped); r > g {
		t.Fatalf("zero-alpha red survived quantization, mean r %d g %d", r, g)
	}

	kept := filepath.Join(dir, "kept.blp")
	if err := ConvertTextureToBlp(EncodeInput{PNG: pngBytes, Opaque: true}, kept); err != nil {
		t.Fatal(err)
	}
	r, g, b := meanBLP1(t, kept)
	if r < 40 || r < g || r < b {
		t.Fatalf("opaque encode lost the red albedo, mean rgb %d %d %d", r, g, b)
	}
}

func meanBLP1(t *testing.T, path string) (int, int, int) {
	t.Helper()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	width := int(uint32(buf[12]) | uint32(buf[13])<<8 | uint32(buf[14])<<16 | uint32(buf[15])<<24)
	height := int(uint32(buf[16]) | uint32(buf[17])<<8 | uint32(buf[18])<<16 | uint32(buf[19])<<24)
	mip := int(uint32(buf[28]) | uint32(buf[29])<<8 | uint32(buf[30])<<16 | uint32(buf[31])<<24)
	n := width * height
	if mip+n > len(buf) {
		t.Fatalf("mip out of range")
	}
	var r, g, b, c int
	step := 1
	if n > 200 {
		step = n / 200
	}
	for i := 0; i < n; i += step {
		p := 156 + int(buf[mip+i])*4
		b += int(buf[p])
		g += int(buf[p+1])
		r += int(buf[p+2])
		c++
	}
	return r / c, g / c, b / c
}
