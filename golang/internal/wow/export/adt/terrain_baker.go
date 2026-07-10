package adt

import (
	"context"
	"math"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	blpfmt "github.com/pqhuy98/wow-converter/internal/formats/blp"
)

// CPUMipTexture holds a mip chain for terrain baking.
type CPUMipTexture struct {
	Mips []MipLevel
}

// MipLevel is one mip level of RGBA data.
type MipLevel struct {
	Data           []byte
	Width, Height  int
}

// BakeMaterial is a terrain bake layer material.
type BakeMaterial struct {
	Scale                     float64
	HeightScale, HeightOffset float64
	DiffuseTex                *CPUMipTexture
	HeightTex                 *CPUMipTexture
}

var bakeTextureCache sync.Map

// LoadBakeTexture loads and decodes a BLP texture for baking.
func LoadBakeTexture(ctx context.Context, getFile func(context.Context, uint32) ([]byte, error), fileDataID uint32) (*CPUMipTexture, error) {
	if cached, ok := bakeTextureCache.Load(fileDataID); ok {
		return cached.(*CPUMipTexture), nil
	}
	raw, err := getFile(ctx, fileDataID)
	if err != nil {
		return nil, err
	}
	data := buffer.From(raw)
	img, err := blpfmt.NewBLPImage(data)
	if err != nil {
		return nil, err
	}
	rgba, err := img.ToUInt8Array(0, 0b1111)
	if err != nil {
		return nil, err
	}
	tex := &CPUMipTexture{Mips: BuildMipChain(rgba, img.ScaledWidth, img.ScaledHeight)}
	bakeTextureCache.Store(fileDataID, tex)
	return tex, nil
}

// ClearBakeTextureCache clears the bake texture cache.
func ClearBakeTextureCache() { bakeTextureCache = sync.Map{} }

// FixChunkAlphaLayers fixes 63x63 alpha edge cases.
func FixChunkAlphaLayers(alphaLayersRaw [][]uint8, fixAlphaMap bool) [][]uint8 {
	alphaLayers := make([][]uint8, len(alphaLayersRaw))
	for i := 1; i < len(alphaLayersRaw); i++ {
		source := alphaLayersRaw[i]
		if fixAlphaMap && len(source) == 64*64 {
			fixed := make([]uint8, 64*64)
			for j := 0; j < 64*64; j++ {
				isLastColumn := j%64 == 63
				isLastRow := j >= 63*64
				switch {
				case isLastColumn && !isLastRow:
					fixed[j] = source[j-1]
				case isLastRow:
					fixed[j] = source[j-64]
				default:
					fixed[j] = source[j]
				}
			}
			source = fixed
		}
		alphaLayers[i] = source
	}
	return alphaLayers
}

// BuildMipChain builds a box-filtered mip chain.
func BuildMipChain(data []byte, width, height int) []MipLevel {
	mips := []MipLevel{{Data: data, Width: width, Height: height}}
	cur := data
	w, h := width, height
	for w > 1 || h > 1 {
		nw := max(1, w>>1)
		nh := max(1, h>>1)
		next := make([]byte, nw*nh*4)
		for y := 0; y < nh; y++ {
			sy0 := min(h-1, y*2)
			sy1 := min(h-1, y*2+1)
			for x := 0; x < nw; x++ {
				sx0 := min(w-1, x*2)
				sx1 := min(w-1, x*2+1)
				i00 := (sy0*w + sx0) * 4
				i10 := (sy0*w + sx1) * 4
				i01 := (sy1*w + sx0) * 4
				i11 := (sy1*w + sx1) * 4
				o := (y*nw + x) * 4
				for c := 0; c < 4; c++ {
					next[o+c] = byte((int(cur[i00+c]) + int(cur[i10+c]) + int(cur[i01+c]) + int(cur[i11+c]) + 2) >> 2)
				}
			}
		}
		mips = append(mips, MipLevel{Data: next, Width: nw, Height: nh})
		cur = next
		w, h = nw, nh
	}
	return mips
}

// ChunkBakeParams configures baking one terrain chunk.
type ChunkBakeParams struct {
	Canvas                      []byte
	CanvasSize                  int
	Indices                     []int
	Vertices                    []float32
	UvsBake                     []float32
	VertexColors                []float32
	Translation                 [2]float64
	TileSize, Zoom              float64
	Layers                      [4]*BakeMaterial
	AlphaLayers                 [][]uint8
}

// BakeChunk rasterizes one map chunk onto its canvas.
func BakeChunk(params ChunkBakeParams) {
	canvas := params.Canvas
	W := params.CanvasSize
	H := params.CanvasSize
	scales := [4]float64{1, 1, 1, 1}
	for i := 0; i < 4; i++ {
		if params.Layers[i] != nil {
			scales[i] = params.Layers[i].Scale
		}
	}
	lods := [4]float64{}
	for i := 0; i < 4; i++ {
		mat := params.Layers[i]
		if mat == nil || mat.DiffuseTex == nil || len(mat.DiffuseTex.Mips) == 0 {
			continue
		}
		base := mat.DiffuseTex.Mips[0]
		texelsPerPx := float64(max(base.Width, base.Height)) * (8 / scales[i]) / float64(W)
		lods[i] = math.Log2(math.Max(texelsPerPx, 1e-9))
	}
	transform := func(vi int) (float64, float64) {
		x := float64(params.Vertices[vi*3])
		z := float64(params.Vertices[vi*3+2])
		cx := ((x+params.Translation[0])/params.TileSize)*2 - 1
		cy := ((((z + params.Translation[1]) / params.TileSize) * 2) - 1) * -1
		ndcX := cx / params.Zoom
		ndcY := cy / params.Zoom
		return ((ndcX + 1) / 2) * float64(W), (1 - (ndcY+1)/2) * float64(H)
	}
	t0, t1, t2, t3 := make([]float64, 4), make([]float64, 4), make([]float64, 4), make([]float64, 4)
	for tri := 0; tri < len(params.Indices); tri += 3 {
		i0, i1, i2 := params.Indices[tri], params.Indices[tri+1], params.Indices[tri+2]
		p0x, p0y := transform(i0)
		p1x, p1y := transform(i1)
		p2x, p2y := transform(i2)
		area := (p1x-p0x)*(p2y-p0y) - (p2x-p0x)*(p1y-p0y)
		if area == 0 {
			continue
		}
		invArea := 1 / area
		minX := max(0, int(math.Floor(min3(p0x, p1x, p2x))))
		maxX := min(W-1, int(math.Ceil(max3(p0x, p1x, p2x))))
		minY := max(0, int(math.Floor(min3(p0y, p1y, p2y))))
		maxY := min(H-1, int(math.Ceil(max3(p0y, p1y, p2y))))
		for yPix := minY; yPix <= maxY; yPix++ {
			sy := float64(yPix) + 0.5
			for xPix := minX; xPix <= maxX; xPix++ {
				sx := float64(xPix) + 0.5
				w0 := ((p1x-sx)*(p2y-sy) - (p2x-sx)*(p1y-sy)) * invArea
				w1 := ((p2x-sx)*(p0y-sy) - (p0x-sx)*(p2y-sy)) * invArea
				w2 := 1 - w0 - w1
				if w0 < 0 || w1 < 0 || w2 < 0 {
					continue
				}
				u := float64(params.UvsBake[i0*2])*w0 + float64(params.UvsBake[i1*2])*w1 + float64(params.UvsBake[i2*2])*w2
				v := float64(params.UvsBake[i0*2+1])*w0 + float64(params.UvsBake[i1*2+1])*w1 + float64(params.UvsBake[i2*2+1])*w2
				vtU, vtV := u*16, v*-16
				vcR := float64(params.VertexColors[i0*4])*w0 + float64(params.VertexColors[i1*4])*w1 + float64(params.VertexColors[i2*4])*w2
				vcG := float64(params.VertexColors[i0*4+1])*w0 + float64(params.VertexColors[i1*4+1])*w1 + float64(params.VertexColors[i2*4+1])*w2
				vcB := float64(params.VertexColors[i0*4+2])*w0 + float64(params.VertexColors[i1*4+2])*w1 + float64(params.VertexColors[i2*4+2])*w2
				modU, modV := glslMod(vtU, 1), glslMod(vtV, 1)
				a0, a1, a2 := 0.0, 0.0, 0.0
				if len(params.AlphaLayers) > 1 && params.AlphaLayers[1] != nil {
					a0 = sampleAlphaLinearClamp(params.AlphaLayers[1], modU, modV)
				}
				if len(params.AlphaLayers) > 2 && params.AlphaLayers[2] != nil {
					a1 = sampleAlphaLinearClamp(params.AlphaLayers[2], modU, modV)
				}
				if len(params.AlphaLayers) > 3 && params.AlphaLayers[3] != nil {
					a2 = sampleAlphaLinearClamp(params.AlphaLayers[3], modU, modV)
				}
				if params.Layers[0] != nil && params.Layers[0].DiffuseTex != nil {
					sampleDiffuse(params.Layers[0].DiffuseTex, vtU*(8/scales[0]), vtV*(8/scales[0]), lods[0], t0)
				} else {
					fill0(t0)
				}
				if params.Layers[1] != nil && params.Layers[1].DiffuseTex != nil {
					sampleDiffuse(params.Layers[1].DiffuseTex, vtU*(8/scales[1]), vtV*(8/scales[1]), lods[1], t1)
				} else {
					fill0(t1)
				}
				if params.Layers[2] != nil && params.Layers[2].DiffuseTex != nil {
					sampleDiffuse(params.Layers[2].DiffuseTex, vtU*(8/scales[2]), vtV*(8/scales[2]), lods[2], t2)
				} else {
					fill0(t2)
				}
				if params.Layers[3] != nil && params.Layers[3].DiffuseTex != nil {
					sampleDiffuse(params.Layers[3].DiffuseTex, vtU*(8/scales[3]), vtV*(8/scales[3]), lods[3], t3)
				} else {
					fill0(t3)
				}
				baseW := 1 - (a0 + a1 + a2)
				r := (t0[0]*baseW+t1[0]*a0+t2[0]*a1+t3[0]*a2) * vcR * 2
				g := (t0[1]*baseW+t1[1]*a0+t2[1]*a1+t3[1]*a2) * vcG * 2
				b := (t0[2]*baseW+t1[2]*a0+t2[2]*a1+t3[2]*a2) * vcB * 2
				o := (yPix*W + xPix) * 4
				canvas[o] = byte(r * 255)
				canvas[o+1] = byte(g * 255)
				canvas[o+2] = byte(b * 255)
				canvas[o+3] = 255
			}
		}
	}
}

func glslMod(x, y float64) float64 { return x - y*math.Floor(x/y) }

func fill0(out []float64) { out[0], out[1], out[2], out[3] = 0, 0, 0, 0 }

func sampleAlphaLinearClamp(layer []uint8, u, v float64) float64 {
	size := 64.0
	fx := u*size - 0.5
	fy := v*size - 0.5
	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	dx := fx - float64(x0)
	dy := fy - float64(y0)
	clamp := func(val int) int {
		if val < 0 {
			return 0
		}
		if val > 63 {
			return 63
		}
		return val
	}
	cx0, cx1 := clamp(x0), clamp(x0+1)
	cy0, cy1 := clamp(y0), clamp(y0+1)
	s00 := float64(layer[cy0*64+cx0])
	s10 := float64(layer[cy0*64+cx1])
	s01 := float64(layer[cy1*64+cx0])
	s11 := float64(layer[cy1*64+cx1])
	return (s00*(1-dx)*(1-dy) + s10*dx*(1-dy) + s01*(1-dx)*dy + s11*dx*dy) / 255
}

func sampleDiffuse(tex *CPUMipTexture, u, v, lod float64, out []float64) {
	mips := tex.Mips
	if lod <= 0 {
		sampleLinearRepeat(mips[0], u, v, out)
		return
	}
	maxLevel := len(mips) - 1
	if lod >= float64(maxLevel) {
		sampleNearestRepeat(mips[maxLevel], u, v, out)
		return
	}
	d1 := int(math.Floor(lod))
	frac := lod - float64(d1)
	a, b := make([]float64, 4), make([]float64, 4)
	sampleNearestRepeat(mips[d1], u, v, a)
	sampleNearestRepeat(mips[d1+1], u, v, b)
	for c := 0; c < 4; c++ {
		out[c] = a[c]*(1-frac) + b[c]*frac
	}
}

func sampleNearestRepeat(mip MipLevel, u, v float64, out []float64) {
	tx := int(math.Floor(glslMod(u, 1) * float64(mip.Width)))
	ty := int(math.Floor(glslMod(v, 1) * float64(mip.Height)))
	if tx >= mip.Width {
		tx = mip.Width - 1
	}
	if ty >= mip.Height {
		ty = mip.Height - 1
	}
	i := (ty*mip.Width + tx) * 4
	out[0] = float64(mip.Data[i]) / 255
	out[1] = float64(mip.Data[i+1]) / 255
	out[2] = float64(mip.Data[i+2]) / 255
	out[3] = float64(mip.Data[i+3]) / 255
}

func sampleLinearRepeat(mip MipLevel, u, v float64, out []float64) {
	fx := glslMod(u, 1)*float64(mip.Width) - 0.5
	fy := glslMod(v, 1)*float64(mip.Height) - 0.5
	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	dx := fx - float64(x0)
	dy := fy - float64(y0)
	wrap := func(val, n int) int { return ((val % n) + n) % n }
	cx0, cx1 := wrap(x0, mip.Width), wrap(x0+1, mip.Width)
	cy0, cy1 := wrap(y0, mip.Height), wrap(y0+1, mip.Height)
	i00 := (cy0*mip.Width + cx0) * 4
	i10 := (cy0*mip.Width + cx1) * 4
	i01 := (cy1*mip.Width + cx0) * 4
	i11 := (cy1*mip.Width + cx1) * 4
	w00 := (1 - dx) * (1 - dy)
	w10 := dx * (1 - dy)
	w01 := (1 - dx) * dy
	w11 := dx * dy
	for c := 0; c < 4; c++ {
		out[c] = (float64(mip.Data[i00+c])*w00 + float64(mip.Data[i10+c])*w10 + float64(mip.Data[i01+c])*w01 + float64(mip.Data[i11+c])*w11) / 255
	}
}

// Rotate180 rotates RGBA image 180 degrees.
func Rotate180(src []byte, size int) []byte {
	out := make([]byte, len(src))
	n := size * size
	for i := 0; i < n; i++ {
		j := n - 1 - i
		out[j*4] = src[i*4]
		out[j*4+1] = src[i*4+1]
		out[j*4+2] = src[i*4+2]
		out[j*4+3] = src[i*4+3]
	}
	return out
}

// ResizeBilinear bilinear-resizes an RGBA image (minimap scaling).
func ResizeBilinear(src []byte, srcW, srcH, dstW, dstH int) []byte {
	out := make([]byte, dstW*dstH*4)
	xRatio := float64(srcW) / float64(dstW)
	yRatio := float64(srcH) / float64(dstH)
	for y := 0; y < dstH; y++ {
		fy := (float64(y)+0.5)*yRatio - 0.5
		y0 := max(0, int(math.Floor(fy)))
		y1 := min(srcH-1, y0+1)
		dy := fy - math.Floor(fy)
		for x := 0; x < dstW; x++ {
			fx := (float64(x)+0.5)*xRatio - 0.5
			x0 := max(0, int(math.Floor(fx)))
			x1 := min(srcW-1, x0+1)
			dx := fx - math.Floor(fx)
			i00 := (y0*srcW + x0) * 4
			i10 := (y0*srcW + x1) * 4
			i01 := (y1*srcW + x0) * 4
			i11 := (y1*srcW + x1) * 4
			o := (y*dstW + x) * 4
			for c := 0; c < 4; c++ {
				out[o+c] = byte(float64(src[i00+c])*(1-dx)*(1-dy) +
					float64(src[i10+c])*dx*(1-dy) +
					float64(src[i01+c])*(1-dx)*dy +
					float64(src[i11+c])*dx*dy)
			}
		}
	}
	return out
}

func min3(a, b, c float64) float64 { return math.Min(a, math.Min(b, c)) }
func max3(a, b, c float64) float64 { return math.Max(a, math.Max(b, c)) }
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
