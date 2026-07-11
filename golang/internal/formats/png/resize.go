package png

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdpng "image/png"
	"math"
	"os"
)

// Dimensions holds PNG width and height.
type Dimensions struct {
	Width  int
	Height int
}

// DecodeRGBA decodes PNG bytes into a flat RGBA buffer and dimensions.
func DecodeRGBA(pngBuffer []byte) ([]byte, int, int, error) {
	img, err := stdpng.Decode(bytes.NewReader(pngBuffer))
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	out := make([]byte, width*height*4)
	if nrgba, ok := img.(*image.NRGBA); ok && bounds.Min.X == 0 && bounds.Min.Y == 0 {
		copy(out, nrgba.Pix[:width*height*4])
		return out, width, height, nil
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.NRGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			i := (y*width + x) * 4
			out[i] = c.R
			out[i+1] = c.G
			out[i+2] = c.B
			out[i+3] = c.A
		}
	}
	return out, width, height, nil
}

// EncodeRGBA encodes flat RGBA bytes as PNG.
func EncodeRGBA(data []byte, width, height int) ([]byte, error) {
	writer := NewWriter(width, height)
	copy(writer.PixelData(), data)
	buf, err := writer.Buffer()
	if err != nil {
		return nil, err
	}
	return buf.Raw(), nil
}

// GetPngDimensions returns PNG dimensions from bytes.
func GetPngDimensions(pngBuffer []byte) (Dimensions, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(pngBuffer))
	if err != nil {
		return Dimensions{}, err
	}
	if cfg.Width == 0 || cfg.Height == 0 {
		return Dimensions{}, fmt.Errorf("invalid image dimensions")
	}
	return Dimensions{Width: cfg.Width, Height: cfg.Height}, nil
}

// ResizePngBytes resizes PNG bytes with separate RGB/alpha channels when alpha is present.
func ResizePngBytes(pngBuffer []byte, targetWidth, targetHeight int) ([]byte, error) {
	data, width, height, err := DecodeRGBA(pngBuffer)
	if err != nil {
		return nil, err
	}
	hasAlpha := false
	for i := 3; i < len(data); i += 4 {
		if data[i] != 255 {
			hasAlpha = true
			break
		}
	}
	var resized []byte
	if hasAlpha {
		rgb := make([]byte, width*height*3)
		alpha := make([]byte, width*height)
		for i := 0; i < width*height; i++ {
			rgb[i*3] = data[i*4]
			rgb[i*3+1] = data[i*4+1]
			rgb[i*3+2] = data[i*4+2]
			alpha[i] = data[i*4+3]
		}
		resizedRGB := resizeGrayPlane(rgb, width, height, 3, targetWidth, targetHeight)
		resizedAlpha := resizeGrayPlane(alpha, width, height, 1, targetWidth, targetHeight)
		resized = make([]byte, targetWidth*targetHeight*4)
		for i := 0; i < targetWidth*targetHeight; i++ {
			resized[i*4] = resizedRGB[i*3]
			resized[i*4+1] = resizedRGB[i*3+1]
			resized[i*4+2] = resizedRGB[i*3+2]
			resized[i*4+3] = resizedAlpha[i]
		}
	} else {
		resized = resizeRGBA(data, width, height, targetWidth, targetHeight)
	}
	return EncodeRGBA(resized, targetWidth, targetHeight)
}

// ResizePngOutside scales PNG to cover target dimensions (sharp fit: outside).
// When alpha is present, RGB and alpha are resized independently (TS resizePng parity).
func ResizePngOutside(pngBuffer []byte, targetWidth, targetHeight int) ([]byte, error) {
	data, width, height, err := DecodeRGBA(pngBuffer)
	if err != nil {
		return nil, err
	}
	scale := math.Max(float64(targetWidth)/float64(width), float64(targetHeight)/float64(height))
	outW := int(math.Ceil(float64(width) * scale))
	outH := int(math.Ceil(float64(height) * scale))
	if outW < 1 {
		outW = 1
	}
	if outH < 1 {
		outH = 1
	}

	hasAlpha := false
	for i := 3; i < len(data); i += 4 {
		if data[i] != 255 {
			hasAlpha = true
			break
		}
	}
	if !hasAlpha {
		resized := resizeRGBA(data, width, height, outW, outH)
		return EncodeRGBA(resized, outW, outH)
	}

	rgb := make([]byte, width*height*3)
	alpha := make([]byte, width*height)
	for i := 0; i < width*height; i++ {
		rgb[i*3] = data[i*4]
		rgb[i*3+1] = data[i*4+1]
		rgb[i*3+2] = data[i*4+2]
		alpha[i] = data[i*4+3]
	}
	resizedRGB := resizeGrayPlane(rgb, width, height, 3, outW, outH)
	resizedAlpha := resizeGrayPlane(alpha, width, height, 1, outW, outH)
	resized := make([]byte, outW*outH*4)
	for i := 0; i < outW*outH; i++ {
		resized[i*4] = resizedRGB[i*3]
		resized[i*4+1] = resizedRGB[i*3+1]
		resized[i*4+2] = resizedRGB[i*3+2]
		resized[i*4+3] = resizedAlpha[i]
	}
	return EncodeRGBA(resized, outW, outH)
}

// ResizePngFill stretches PNG to exact target dimensions.
func ResizePngFill(pngBuffer []byte, targetWidth, targetHeight int) ([]byte, error) {
	data, width, height, err := DecodeRGBA(pngBuffer)
	if err != nil {
		return nil, err
	}
	resized := resizeRGBA(data, width, height, targetWidth, targetHeight)
	return EncodeRGBA(resized, targetWidth, targetHeight)
}

// IsAbnormalTransparency detects odd-column fully-transparent alpha patterns.
func IsAbnormalTransparency(pngBuffer []byte) (bool, error) {
	data, width, height, err := DecodeRGBA(pngBuffer)
	if err != nil {
		return false, err
	}
	for x := 1; x < width; x += 2 {
		for y := 0; y < height; y++ {
			if data[(y*width+x)*4+3] != 0 {
				return false, nil
			}
		}
	}
	return true, nil
}

// RemoveAlphaRGB clears RGB on fully transparent pixels.
func RemoveAlphaRGB(data []byte) {
	for i := 0; i < len(data); i += 4 {
		if data[i+3] <= 1 {
			data[i] = 0
			data[i+1] = 0
			data[i+2] = 0
		}
	}
}

func resizeRGBA(src []byte, sw, sh, dw, dh int) []byte {
	return resizeGrayPlane(src, sw, sh, 4, dw, dh)
}

func resizeGrayPlane(src []byte, sw, sh, channels, dw, dh int) []byte {
	dst := make([]byte, dw*dh*channels)
	if sw == 0 || sh == 0 || dw == 0 || dh == 0 {
		return dst
	}
	xRatio := float64(sw) / float64(dw)
	yRatio := float64(sh) / float64(dh)
	for y := 0; y < dh; y++ {
		fy := (float64(y)+0.5)*yRatio - 0.5
		y0 := maxInt(0, int(math.Floor(fy)))
		y1 := minInt(sh-1, y0+1)
		dy := fy - math.Floor(fy)
		for x := 0; x < dw; x++ {
			fx := (float64(x)+0.5)*xRatio - 0.5
			x0 := maxInt(0, int(math.Floor(fx)))
			x1 := minInt(sw-1, x0+1)
			dx := fx - math.Floor(fx)
			i00 := (y0*sw + x0) * channels
			i10 := (y0*sw + x1) * channels
			i01 := (y1*sw + x0) * channels
			i11 := (y1*sw + x1) * channels
			o := (y*dw + x) * channels
			for c := 0; c < channels; c++ {
				dst[o+c] = byte(
					float64(src[i00+c])*(1-dx)*(1-dy) +
						float64(src[i10+c])*dx*(1-dy) +
						float64(src[i01+c])*(1-dx)*dy +
						float64(src[i11+c])*dx*dy,
				)
			}
		}
	}
	return dst
}

// GetPngDimensionsFromFile reads PNG dimensions from a file path.
func GetPngDimensionsFromFile(path string) (Dimensions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Dimensions{}, err
	}
	return GetPngDimensions(data)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
