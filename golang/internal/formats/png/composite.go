package png

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/png"
	"os"
)

// Draw describes an overlay region as fractions of the base texture.
type Draw struct {
	PngPath string
	X       float64
	Y       float64
	Width   float64
	Height  float64
}

// DrawPngsOnBasePng composites overlay PNGs onto a base PNG file.
func DrawPngsOnBasePng(basePngPath string, draws []Draw) ([]byte, error) {
	baseFile, err := os.Open(basePngPath)
	if err != nil {
		return nil, err
	}
	defer baseFile.Close()
	baseImg, _, err := image.Decode(baseFile)
	if err != nil {
		return nil, err
	}
	bounds := baseImg.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, baseImg, bounds.Min, draw.Src)

	if len(draws) == 0 {
		return encodeImageRGBA(rgba)
	}

	for _, d := range draws {
		overlayData, err := os.ReadFile(d.PngPath)
		if err != nil {
			continue
		}
		targetW := maxInt(1, int(float64(bounds.Dx())*d.Width+0.5))
		targetH := maxInt(1, int(float64(bounds.Dy())*d.Height+0.5))
		left := int(float64(bounds.Dx())*d.X + 0.5)
		top := int(float64(bounds.Dy())*d.Y + 0.5)

		var resized []byte
		abnormal, err := IsAbnormalTransparency(overlayData)
		if err == nil && abnormal {
			noAlpha, err := removeAlphaChannel(overlayData)
			if err != nil {
				continue
			}
			resized, err = ResizePngOutside(noAlpha, targetW, targetH)
		} else {
			resized, err = ResizePngOutside(overlayData, targetW, targetH)
		}
		if err != nil {
			continue
		}
		overlayImg, _, err := image.Decode(bytes.NewReader(resized))
		if err != nil {
			continue
		}
		destRect := image.Rect(left, top, left+overlayImg.Bounds().Dx(), top+overlayImg.Bounds().Dy())
		draw.Draw(rgba, destRect, overlayImg, overlayImg.Bounds().Min, draw.Over)
	}
	return encodeImageRGBA(rgba)
}

func removeAlphaChannel(pngBuffer []byte) ([]byte, error) {
	data, width, height, err := DecodeRGBA(pngBuffer)
	if err != nil {
		return nil, err
	}
	for i := 3; i < len(data); i += 4 {
		data[i] = 255
	}
	return EncodeRGBA(data, width, height)
}

func encodeImageRGBA(img *image.RGBA) ([]byte, error) {
	data := make([]byte, img.Bounds().Dx()*img.Bounds().Dy()*4)
	for y := 0; y < img.Bounds().Dy(); y++ {
		copy(data[y*img.Bounds().Dx()*4:(y+1)*img.Bounds().Dx()*4], img.Pix[y*img.Stride : (y+1)*img.Stride][:img.Bounds().Dx()*4])
	}
	return EncodeRGBA(data, img.Bounds().Dx(), img.Bounds().Dy())
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
