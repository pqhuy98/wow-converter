package blp

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"log"
	stdpng "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	pngwriter "github.com/pqhuy98/wow-converter/internal/formats/png"
)

const blp1HeaderSize = 156

var logJSFallbackOnce sync.Once

// EncodeInput is input for a single BLP1 encode task.
type EncodeInput struct {
	PNG      []byte
	BLP2     []byte
	ResizeTo *Size
}

// Size holds width and height dimensions.
type Size struct {
	Width  int
	Height int
}

// ConvertTextureToBlp decodes BLP2 or PNG input and writes WC3 BLP1.
func ConvertTextureToBlp(input EncodeInput, blpPath string) error {
	pngData := input.PNG
	if pngData == nil {
		if len(input.BLP2) == 0 {
			return fmt.Errorf("convertTextureToBlp: either png or blp2 input is required")
		}
		img, err := NewBLPImage(buffer.From(input.BLP2))
		if err != nil {
			return err
		}
		buf, err := img.ToPNG(0b1111, 0)
		if err != nil {
			return err
		}
		pngData = buf.Raw()
	}
	if input.ResizeTo != nil {
		resized, err := pngwriter.ResizePngBytes(pngData, input.ResizeTo.Width, input.ResizeTo.Height)
		if err != nil {
			log.Printf("Failed to resize PNG, proceeding without resize: %s: %v", blpPath, err)
		} else {
			pngData = resized
		}
	}
	return ConvertPngToBlp(pngData, blpPath)
}

// ConvertPngToBlp converts PNG bytes to a BLP1 file using the native C++ encoder
// when CGO is enabled, otherwise the Go JS-fallback path.
func ConvertPngToBlp(pngBufferOriginal []byte, blpPath string) error {
	pngBuffer, err := EnsureOpaqueIfAllAlphaZero(pngBufferOriginal)
	if err != nil {
		return err
	}
	if NativeEncoderAvailable() {
		if err := encodeNative(pngBuffer, blpPath); err != nil {
			return err
		}
		return nil
	}
	logJSFallbackOnce.Do(func() {
		log.Printf("Failed to load PNG->BLP's C++ native binding, will fallback to slower JavaScript implementation")
	})
	return Png2BlpJS(pngBuffer, blpPath)
}

// Png2BlpJS encodes PNG bytes as WC3 BLP1 (JavaScript fallback path).
func Png2BlpJS(pngBuffer []byte, distPath string) error {
	data, width, height, err := decodePNGToRGBA(pngBuffer)
	if err != nil {
		return err
	}
	pixelCount := width * height

	indices0 := make([]byte, pixelCount)
	alpha0 := make([]byte, pixelCount)

	rgbToIndexFast := make(map[int]int, 256)
	paletteBufferFast := make([]byte, 256*4)
	paletteSizeFast := 0
	exceededFastPath := false

	for i := 0; i < pixelCount; i++ {
		r := data[i*4]
		g := data[i*4+1]
		b := data[i*4+2]
		a := data[i*4+3]
		alpha0[i] = a

		key := rgbKey(r, g, b)
		if _, ok := rgbToIndexFast[key]; !ok {
			if paletteSizeFast == 256 {
				exceededFastPath = true
				continue
			}
			idx := paletteSizeFast
			rgbToIndexFast[key] = idx
			p := idx * 4
			paletteBufferFast[p] = b
			paletteBufferFast[p+1] = g
			paletteBufferFast[p+2] = r
			paletteBufferFast[p+3] = 0xFF
			paletteSizeFast++
		}
	}

	var paletteBuffer []byte
	var paletteSize int

	if !exceededFastPath {
		paletteBuffer = paletteBufferFast
		paletteSize = paletteSizeFast
		for i := 0; i < pixelCount; i++ {
			r := data[i*4]
			g := data[i*4+1]
			b := data[i*4+2]
			indices0[i] = byte(rgbToIndexFast[rgbKey(r, g, b)])
		}
	} else {
		paletteBuffer, paletteSize = quantizeNativeKMeans(data, width, height, indices0, alpha0)
	}

	lut64 := buildLut64FromPalette(paletteBuffer, paletteSize)
	mipDims := planMipDims(width, height)
	mipCount := len(mipDims)

	header := make([]byte, blp1HeaderSize)
	copy(header[0:4], []byte("BLP1"))
	putU32LE(header, 4, 1)
	putU32LE(header, 8, 8)
	putU32LE(header, 12, uint32(width))
	putU32LE(header, 16, uint32(height))
	putU32LE(header, 20, 0)
	if mipCount > 1 {
		putU32LE(header, 24, 1)
	}

	offsetPos := 28
	sizePos := offsetPos + 64
	pixelDataOffset := blp1HeaderSize + 1024

	totalMipBytes := 0
	for _, d := range mipDims {
		totalMipBytes += d.w * d.h * 2
	}

	out := make([]byte, blp1HeaderSize+1024+totalMipBytes)
	copy(out[blp1HeaderSize:], paletteBuffer)

	writeOffset := pixelDataOffset
	currentRGBA := data
	cw := width
	ch := height

	for level := 0; level < mipCount; level++ {
		pixels := cw * ch
		mipBytes := pixels * 2
		putU32LE(header, offsetPos+level*4, uint32(writeOffset))
		putU32LE(header, sizePos+level*4, uint32(mipBytes))

		if level == 0 {
			copy(out[writeOffset:], indices0)
			copy(out[writeOffset+pixels:], alpha0)
		} else {
			idxDst := out[writeOffset : writeOffset+pixels]
			aDst := out[writeOffset+pixels : writeOffset+mipBytes]
			for i := 0; i < pixels; i++ {
				base := i * 4
				r := currentRGBA[base]
				g := currentRGBA[base+1]
				b := currentRGBA[base+2]
				a := currentRGBA[base+3]
				r6 := r >> 2
				g6 := g >> 2
				b6 := b >> 2
				idxDst[i] = lut64[(int(r6)<<12)|(int(g6)<<6)|int(b6)]
				aDst[i] = a
			}
		}

		writeOffset += mipBytes
		if level+1 < mipCount {
			next := downsample2x2SeparateAlpha(currentRGBA, cw, ch)
			currentRGBA = next.rgba
			cw = next.w
			ch = next.h
		}
	}

	copy(out, header)

	if err := os.MkdirAll(filepath.Dir(distPath), 0o755); err != nil {
		// Match TS: ignore mkdir errors.
	}
	return os.WriteFile(distPath, out, 0o644)
}

// EnsureOpaqueIfAllAlphaZero forces alpha to 255 when every pixel is fully transparent.
func EnsureOpaqueIfAllAlphaZero(pngBuffer []byte) ([]byte, error) {
	data, width, height, err := decodePNGToRGBA(pngBuffer)
	if err != nil {
		return nil, err
	}
	total := width * height
	for i := 0; i < total; i++ {
		if data[i*4+3] != 0 {
			return pngBuffer, nil
		}
	}
	for i := 0; i < total; i++ {
		data[i*4+3] = 255
	}
	return encodeRGBAtoPNG(data, width, height)
}

type mipDim struct {
	w, h int
}

func planMipDims(width, height int) []mipDim {
	dims := make([]mipDim, 0, 16)
	mw, mh := width, height
	for level := 0; level < 16; level++ {
		dims = append(dims, mipDim{w: mw, h: mh})
		if mw == 1 && mh == 1 {
			break
		}
		mw = int(math.Max(1, math.Ceil(float64(mw)/2)))
		mh = int(math.Max(1, math.Ceil(float64(mh)/2)))
	}
	return dims
}

func rgbKey(r, g, b byte) int {
	return (int(r) << 16) | (int(g) << 8) | int(b)
}

func dist2Bt709(r1, g1, b1, r2, g2, b2 float64) float64 {
	const wr = 0.2126
	const wg = 0.7152
	const wb = 0.0722
	dr := r1 - r2
	dg := g1 - g2
	db := b1 - b2
	return wr*dr*dr + wg*dg*dg + wb*db*db
}

func buildLut64FromPalette(paletteBuffer []byte, paletteSize int) []byte {
	count := paletteSize
	if count < 1 {
		count = 1
	}
	if count > 256 {
		count = 256
	}

	pr := make([]float64, count)
	pg := make([]float64, count)
	pb := make([]float64, count)
	for i := 0; i < count; i++ {
		p := i * 4
		pb[i] = float64(paletteBuffer[p])
		pg[i] = float64(paletteBuffer[p+1])
		pr[i] = float64(paletteBuffer[p+2])
	}

	lut := make([]byte, 64*64*64)
	idx := 0
	for rr := 0; rr < 64; rr++ {
		rSrgb := float64((rr << 2) | 2)
		for gg := 0; gg < 64; gg++ {
			gSrgb := float64((gg << 2) | 2)
			for bb := 0; bb < 64; bb++ {
				bSrgb := float64((bb << 2) | 2)
				best := 0
				bestD := math.MaxFloat64
				for k := 0; k < count; k++ {
					d := dist2Bt709(rSrgb, gSrgb, bSrgb, pr[k], pg[k], pb[k])
					if d < bestD {
						bestD = d
						best = k
					}
				}
				lut[idx] = byte(best)
				idx++
			}
		}
	}
	return lut
}

func downsample2x2SeparateAlpha(src []byte, sw, sh int) struct {
	rgba []byte
	w, h int
} {
	dw := int(math.Max(1, math.Ceil(float64(sw)/2)))
	dh := int(math.Max(1, math.Ceil(float64(sh)/2)))
	dst := make([]byte, dw*dh*4)

	for y := 0; y < dh; y++ {
		sy0 := y * 2
		if sy0 > sh-1 {
			sy0 = sh - 1
		}
		sy1 := sy0 + 1
		if sy1 > sh-1 {
			sy1 = sh - 1
		}
		for x := 0; x < dw; x++ {
			sx0 := x * 2
			if sx0 > sw-1 {
				sx0 = sw - 1
			}
			sx1 := sx0 + 1
			if sx1 > sw-1 {
				sx1 = sw - 1
			}

			i00 := (sy0*sw + sx0) * 4
			i10 := (sy0*sw + sx1) * 4
			i01 := (sy1*sw + sx0) * 4
			i11 := (sy1*sw + sx1) * 4

			sumR := int(src[i00]) + int(src[i10]) + int(src[i01]) + int(src[i11])
			sumG := int(src[i00+1]) + int(src[i10+1]) + int(src[i01+1]) + int(src[i11+1])
			sumB := int(src[i00+2]) + int(src[i10+2]) + int(src[i01+2]) + int(src[i11+2])
			sumA := int(src[i00+3]) + int(src[i10+3]) + int(src[i01+3]) + int(src[i11+3])

			di := (y*dw + x) * 4
			dst[di] = byte((sumR + 2) >> 2)
			dst[di+1] = byte((sumG + 2) >> 2)
			dst[di+2] = byte((sumB + 2) >> 2)
			dst[di+3] = byte((sumA + 2) >> 2)
		}
	}

	return struct {
		rgba []byte
		w, h int
	}{rgba: dst, w: dw, h: dh}
}

func putU32LE(buf []byte, offset int, value uint32) {
	buf[offset] = byte(value)
	buf[offset+1] = byte(value >> 8)
	buf[offset+2] = byte(value >> 16)
	buf[offset+3] = byte(value >> 24)
}

func decodePNGToRGBA(pngBuffer []byte) ([]byte, int, int, error) {
	img, err := stdpng.Decode(bytes.NewReader(pngBuffer))
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	out := make([]byte, width*height*4)

	if nrgba, ok := img.(*image.NRGBA); ok {
		for y := 0; y < height; y++ {
			src := nrgba.Pix[(bounds.Min.Y+y-nrgba.Rect.Min.Y)*nrgba.Stride+(bounds.Min.X-nrgba.Rect.Min.X)*4:]
			copy(out[y*width*4:(y+1)*width*4], src[:width*4])
		}
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

func encodeRGBAtoPNG(data []byte, width, height int) ([]byte, error) {
	return pngwriter.EncodeRGBA(data, width, height)
}

type colorBox struct {
	pixels [][3]byte
}

type blpRGBF struct {
	r, g, b float32
}

type blpSampleColor struct {
	r, g, b, w float32
}

func quantizeNativeKMeans(data []byte, width, height int, indices, alpha []byte) ([]byte, int) {
	const (
		histBits  = 5
		histN     = 1 << histBits
		histSize  = histN * histN * histN
		maxIters  = 12
		maxColors = 256
	)

	exactMap := make(map[int]byte, 512)
	exactKeys := make([]int, 0, maxColors)
	exactOK := true
	histR := make([]float32, histSize)
	histG := make([]float32, histSize)
	histB := make([]float32, histSize)
	histW := make([]float32, histSize)
	anyVisible := false

	pixelCount := width * height
	for i := 0; i < pixelCount; i++ {
		r := data[i*4]
		g := data[i*4+1]
		b := data[i*4+2]
		a := data[i*4+3]
		alpha[i] = a

		key := rgbKey(r, g, b)
		if exactOK {
			if _, ok := exactMap[key]; !ok {
				if len(exactKeys) >= maxColors {
					exactOK = false
				} else {
					exactMap[key] = byte(len(exactKeys))
					exactKeys = append(exactKeys, key)
				}
			}
		}

		if a > 0 {
			anyVisible = true
			weight := float32(a) / 255.0
			hi := (int(r)>>(8-histBits))<<(2*histBits) |
				(int(g)>>(8-histBits))<<histBits |
				(int(b) >> (8 - histBits))
			histR[hi] += float32(r) * weight
			histG[hi] += float32(g) * weight
			histB[hi] += float32(b) * weight
			histW[hi] += weight
		}
	}

	paletteBuffer := make([]byte, 256*4)
	if exactOK && len(exactKeys) > 0 {
		for _, key := range exactKeys {
			idx := int(exactMap[key])
			p := idx * 4
			paletteBuffer[p] = byte(key)
			paletteBuffer[p+1] = byte(key >> 8)
			paletteBuffer[p+2] = byte(key >> 16)
			paletteBuffer[p+3] = 0xFF
		}
		for i := 0; i < pixelCount; i++ {
			indices[i] = exactMap[rgbKey(data[i*4], data[i*4+1], data[i*4+2])]
		}
		return paletteBuffer, len(exactKeys)
	}

	if !anyVisible {
		paletteBuffer[3] = 0xFF
		for i := range indices {
			indices[i] = 0
		}
		return paletteBuffer, 1
	}

	samples := make([]blpSampleColor, 0, 4096)
	for hi := 0; hi < histSize; hi++ {
		weight := histW[hi]
		if weight <= 0 {
			continue
		}
		inv := 1.0 / weight
		samples = append(samples, blpSampleColor{
			r: histR[hi] * inv,
			g: histG[hi] * inv,
			b: histB[hi] * inv,
			w: weight,
		})
	}

	kCount := len(samples)
	if kCount > maxColors {
		kCount = maxColors
	}
	centers := make([]blpRGBF, 0, kCount)
	centers = append(centers, blpRGBF{samples[0].r, samples[0].g, samples[0].b})
	minD2 := make([]float32, len(samples))
	for i := range samples {
		minD2[i] = dist2Bt709f(samples[i].r, samples[i].g, samples[i].b, centers[0].r, centers[0].g, centers[0].b)
	}
	for len(centers) < kCount {
		farIdx := 0
		farVal := float32(-1)
		for i := range samples {
			v := minD2[i] * samples[i].w
			if v > farVal {
				farVal = v
				farIdx = i
			}
		}
		next := blpRGBF{samples[farIdx].r, samples[farIdx].g, samples[farIdx].b}
		centers = append(centers, next)
		for i := range samples {
			d2 := dist2Bt709f(samples[i].r, samples[i].g, samples[i].b, next.r, next.g, next.b)
			if d2 < minD2[i] {
				minD2[i] = d2
			}
		}
	}

	counts := make([]float32, kCount)
	sums := make([]blpRGBF, kCount)
	for iter := 0; iter < maxIters; iter++ {
		for i := range counts {
			counts[i] = 0
			sums[i] = blpRGBF{}
		}

		for i := range samples {
			s := samples[i]
			bestK := 0
			bestD := float32(math.MaxFloat32)
			for k := 0; k < kCount; k++ {
				d := dist2Bt709f(s.r, s.g, s.b, centers[k].r, centers[k].g, centers[k].b)
				if d < bestD {
					bestD = d
					bestK = k
				}
			}
			weight := s.w
			counts[bestK] += weight
			sums[bestK].r += s.r * weight
			sums[bestK].g += s.g * weight
			sums[bestK].b += s.b * weight
		}

		maxShift2 := float32(0)
		for k := 0; k < kCount; k++ {
			if counts[k] <= 1e-6 {
				s := (k * 9973) % len(samples)
				centers[k] = blpRGBF{samples[s].r, samples[s].g, samples[s].b}
				maxShift2 = math.MaxFloat32
				continue
			}
			inv := 1.0 / counts[k]
			nr := sums[k].r * inv
			ng := sums[k].g * inv
			nb := sums[k].b * inv
			shift2 := dist2Bt709f(nr, ng, nb, centers[k].r, centers[k].g, centers[k].b)
			if shift2 > maxShift2 {
				maxShift2 = shift2
			}
			centers[k] = blpRGBF{nr, ng, nb}
		}
		if maxShift2 < 0.25 {
			break
		}
	}

	paletteRGB := make([]blpRGBF, 0, maxColors)
	for k := 0; k < kCount && len(paletteRGB) < maxColors; k++ {
		if counts[k] <= 1e-6 {
			continue
		}
		r := clampFloat32ToByte(centers[k].r)
		g := clampFloat32ToByte(centers[k].g)
		b := clampFloat32ToByte(centers[k].b)
		p := len(paletteRGB) * 4
		paletteBuffer[p] = b
		paletteBuffer[p+1] = g
		paletteBuffer[p+2] = r
		paletteBuffer[p+3] = 0xFF
		paletteRGB = append(paletteRGB, blpRGBF{float32(r), float32(g), float32(b)})
	}
	if len(paletteRGB) == 0 {
		paletteBuffer[3] = 0xFF
		paletteRGB = append(paletteRGB, blpRGBF{})
	}

	lut64 := buildLut64FromPalette(paletteBuffer, len(paletteRGB))
	ditherToPalette(data, width, height, paletteRGB, lut64, indices)
	return paletteBuffer, len(paletteRGB)
}

func dist2Bt709f(r1, g1, b1, r2, g2, b2 float32) float32 {
	const wr = float32(0.2126)
	const wg = float32(0.7152)
	const wb = float32(0.0722)
	dr := r1 - r2
	dg := g1 - g2
	db := b1 - b2
	return wr*dr*dr + wg*dg*dg + wb*db*db
}

func clampFloat32ToByte(v float32) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

func ditherToPalette(data []byte, width, height int, paletteRGB []blpRGBF, lut64 []byte, indices []byte) {
	errR := make([]float32, width+2)
	errG := make([]float32, width+2)
	errB := make([]float32, width+2)
	nextErrR := make([]float32, width+2)
	nextErrG := make([]float32, width+2)
	nextErrB := make([]float32, width+2)

	for y := 0; y < height; y++ {
		rtl := y%2 == 1
		if !rtl {
			for x := 0; x < width; x++ {
				ditherPixel(data, width, x, y, paletteRGB, lut64, indices, errR, errG, errB, nextErrR, nextErrG, nextErrB, false)
			}
		} else {
			for xi := 0; xi < width; xi++ {
				x := width - 1 - xi
				ditherPixel(data, width, x, y, paletteRGB, lut64, indices, errR, errG, errB, nextErrR, nextErrG, nextErrB, true)
			}
		}
		errR, nextErrR = nextErrR, errR
		errG, nextErrG = nextErrG, errG
		errB, nextErrB = nextErrB, errB
		for i := range nextErrR {
			nextErrR[i] = 0
			nextErrG[i] = 0
			nextErrB[i] = 0
		}
	}
}

func ditherPixel(data []byte, width, x, y int, paletteRGB []blpRGBF, lut64 []byte, indices []byte, errR, errG, errB, nextErrR, nextErrG, nextErrB []float32, rtl bool) {
	i := (y*width + x) * 4
	a := float32(data[i+3]) / 255.0
	r := clampFloat32(float32(data[i])+errR[x+1], 0, 255)
	g := clampFloat32(float32(data[i+1])+errG[x+1], 0, 255)
	b := clampFloat32(float32(data[i+2])+errB[x+1], 0, 255)
	best := int(lut64[(int(byte(r)>>2)<<12)|(int(byte(g)>>2)<<6)|int(byte(b)>>2)])
	indices[y*width+x] = byte(best)

	q := paletteRGB[best]
	er := (r - q.r) * a
	eg := (g - q.g) * a
	eb := (b - q.b) * a
	if !rtl {
		errR[x+2] += er * (7.0 / 16.0)
		errG[x+2] += eg * (7.0 / 16.0)
		errB[x+2] += eb * (7.0 / 16.0)
		nextErrR[x+0] += er * (3.0 / 16.0)
		nextErrG[x+0] += eg * (3.0 / 16.0)
		nextErrB[x+0] += eb * (3.0 / 16.0)
		nextErrR[x+1] += er * (5.0 / 16.0)
		nextErrG[x+1] += eg * (5.0 / 16.0)
		nextErrB[x+1] += eb * (5.0 / 16.0)
		nextErrR[x+2] += er * (1.0 / 16.0)
		nextErrG[x+2] += eg * (1.0 / 16.0)
		nextErrB[x+2] += eb * (1.0 / 16.0)
		return
	}
	errR[x+0] += er * (7.0 / 16.0)
	errG[x+0] += eg * (7.0 / 16.0)
	errB[x+0] += eb * (7.0 / 16.0)
	nextErrR[x+2] += er * (3.0 / 16.0)
	nextErrG[x+2] += eg * (3.0 / 16.0)
	nextErrB[x+2] += eb * (3.0 / 16.0)
	nextErrR[x+1] += er * (5.0 / 16.0)
	nextErrG[x+1] += eg * (5.0 / 16.0)
	nextErrB[x+1] += eb * (5.0 / 16.0)
	nextErrR[x+0] += er * (1.0 / 16.0)
	nextErrG[x+0] += eg * (1.0 / 16.0)
	nextErrB[x+0] += eb * (1.0 / 16.0)
}

func clampFloat32(v, minV, maxV float32) float32 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func quantizeRGB(data []byte, width, height int) ([]byte, int) {
	pixels := make([][3]byte, width*height)
	for i := 0; i < len(pixels); i++ {
		pixels[i] = [3]byte{data[i*4], data[i*4+1], data[i*4+2]}
	}

	boxes := []colorBox{{pixels: pixels}}
	for len(boxes) < 256 {
		bestIdx := -1
		bestRange := -1
		for i, box := range boxes {
			if len(box.pixels) < 2 {
				continue
			}
			rng := boxRange(box.pixels)
			if rng > bestRange {
				bestRange = rng
				bestIdx = i
			}
		}
		if bestIdx < 0 {
			break
		}
		left, right := splitBox(boxes[bestIdx].pixels)
		boxes = append(boxes[:bestIdx], append([]colorBox{{pixels: left}, {pixels: right}}, boxes[bestIdx+1:]...)...)
	}

	paletteBuffer := make([]byte, 256*4)
	paletteSize := 0
	for i, box := range boxes {
		if i >= 256 {
			break
		}
		r, g, b := averageColor(box.pixels)
		p := i * 4
		paletteBuffer[p] = b
		paletteBuffer[p+1] = g
		paletteBuffer[p+2] = r
		paletteBuffer[p+3] = 0xFF
		paletteSize++
	}
	return paletteBuffer, paletteSize
}

func boxRange(pixels [][3]byte) int {
	minR, minG, minB := 255, 255, 255
	maxR, maxG, maxB := 0, 0, 0
	for _, p := range pixels {
		if int(p[0]) < minR {
			minR = int(p[0])
		}
		if int(p[1]) < minG {
			minG = int(p[1])
		}
		if int(p[2]) < minB {
			minB = int(p[2])
		}
		if int(p[0]) > maxR {
			maxR = int(p[0])
		}
		if int(p[1]) > maxG {
			maxG = int(p[1])
		}
		if int(p[2]) > maxB {
			maxB = int(p[2])
		}
	}
	return max(maxR-minR, max(maxG-minG, max(maxB-minB)))
}

func splitBox(pixels [][3]byte) ([][3]byte, [][3]byte) {
	minR, minG, minB := 255, 255, 255
	maxR, maxG, maxB := 0, 0, 0
	for _, p := range pixels {
		if int(p[0]) < minR {
			minR = int(p[0])
		}
		if int(p[1]) < minG {
			minG = int(p[1])
		}
		if int(p[2]) < minB {
			minB = int(p[2])
		}
		if int(p[0]) > maxR {
			maxR = int(p[0])
		}
		if int(p[1]) > maxG {
			maxG = int(p[1])
		}
		if int(p[2]) > maxB {
			maxB = int(p[2])
		}
	}
	rRange := maxR - minR
	gRange := maxG - minG
	bRange := maxB - minB

	channel := 0
	if gRange >= rRange && gRange >= bRange {
		channel = 1
	} else if bRange >= rRange && bRange >= gRange {
		channel = 2
	}

	sort.Slice(pixels, func(i, j int) bool {
		return pixels[i][channel] < pixels[j][channel]
	})
	mid := len(pixels) / 2
	return pixels[:mid], pixels[mid:]
}

func averageColor(pixels [][3]byte) (r, g, b byte) {
	if len(pixels) == 0 {
		return 0, 0, 0
	}
	var sr, sg, sb int
	for _, p := range pixels {
		sr += int(p[0])
		sg += int(p[1])
		sb += int(p[2])
	}
	n := len(pixels)
	return byte(sr / n), byte(sg / n), byte(sb / n)
}
