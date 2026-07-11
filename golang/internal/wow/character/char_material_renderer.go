package character

import (
	"context"
	"math"
	"strconv"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/formats/png"
)

// TextureLoader loads raw BLP bytes for a file data id.
type TextureLoader func(ctx context.Context, fileDataID int) ([]byte, error)

// MaterialRenderer bakes character customization textures on the CPU.
type MaterialRenderer struct {
	TextureTargets []textureTarget
	Width          int
	Height         int
	Canvas         []uint8
	loadTexture    TextureLoader
}

type textureTarget struct {
	ID           int
	Section      CharComponentTextureSection
	Material     ChrModelMaterialRow
	TextureLayer ChrModelTextureLayerRow
	CustMaterial ChrCustomizationMaterialEntry
	Texture      cpuTexture
	Filename     string
}

type cpuTexture struct {
	Data   []uint8
	Width  int
	Height int
}

// InitMaterialRenderer is a no-op mirroring the TS init hook.
func InitMaterialRenderer() {}

// NewMaterialRenderer creates a renderer for one texture type canvas.
func NewMaterialRenderer(textureType int, width, height int, load TextureLoader) *MaterialRenderer {
	_ = textureType
	return &MaterialRenderer{
		Width: width, Height: height,
		Canvas:      make([]uint8, width*height*4),
		loadTexture: load,
	}
}

func (r *MaterialRenderer) Init() { r.Reset() }

func (r *MaterialRenderer) Reset() {
	r.TextureTargets = nil
	r.clearCanvas()
}

// Dispose releases renderer resources.
func (r *MaterialRenderer) Dispose() { r.Reset() }

func (r *MaterialRenderer) clearCanvas() {
	for i := range r.Canvas {
		r.Canvas[i] = 0
	}
}

// GetPNG returns baked canvas as PNG bytes (raw framebuffer pixels, no unpremultiply).
func (r *MaterialRenderer) GetPNG() ([]byte, error) {
	writer := png.NewWriter(r.Width, r.Height)
	copy(writer.PixelData(), r.Canvas)
	buf, err := writer.Buffer()
	if err != nil {
		return nil, err
	}
	return buf.Raw(), nil
}

// FirstFilename returns the first loaded texture filename, if any.
func (r *MaterialRenderer) FirstFilename() string {
	for _, t := range r.TextureTargets {
		if t.ID == 1 && t.Filename != "" {
			return t.Filename
		}
	}
	for _, t := range r.TextureTargets {
		if t.Filename != "" {
			return t.Filename
		}
	}
	return ""
}

// SetTextureTarget loads and queues one bake layer.
func (r *MaterialRenderer) SetTextureTarget(
	ctx context.Context,
	custMaterial ChrCustomizationMaterialEntry,
	section CharComponentTextureSection,
	material ChrModelMaterialRow,
	textureLayer ChrModelTextureLayerRow,
	useAlpha bool,
	filenameOverride string,
) error {
	filename := filenameOverride
	tex, err := r.loadTextureBytes(ctx, custMaterial.FileDataID, useAlpha)
	if err != nil {
		return err
	}
	r.TextureTargets = append(r.TextureTargets, textureTarget{
		ID:      custMaterial.ChrModelTextureTargetID,
		Section: section, Material: material, TextureLayer: textureLayer,
		CustMaterial: custMaterial, Texture: tex, Filename: filename,
	})
	r.update()
	return nil
}

func (r *MaterialRenderer) loadTextureBytes(ctx context.Context, fileDataID int, useAlpha bool) (cpuTexture, error) {
	raw, err := r.loadTexture(ctx, fileDataID)
	if err != nil {
		return cpuTexture{}, err
	}
	img, err := blp.NewBLPImage(buffer.From(raw))
	if err != nil {
		return cpuTexture{}, err
	}
	mask := byte(0b0111)
	if useAlpha {
		mask = 0b1111
	}
	data, err := img.ToUInt8Array(0, mask)
	if err != nil {
		return cpuTexture{}, err
	}
	return cpuTexture{Data: data, Width: int(img.Width), Height: int(img.Height)}, nil
}

func (r *MaterialRenderer) update() {
	r.clearCanvas()
	sorted := append([]textureTarget(nil), r.TextureTargets...)
	sortTargetsByID(sorted)
	seen := map[string]struct{}{}
	for _, layer := range sorted {
		key := fmtKey(layer)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		r.drawLayer(layer)
	}
}

func sortTargetsByID(targets []textureTarget) {
	for i := 1; i < len(targets); i++ {
		for j := i; j > 0 && targets[j].ID < targets[j-1].ID; j-- {
			targets[j], targets[j-1] = targets[j-1], targets[j]
		}
	}
}

func fmtKey(layer textureTarget) string {
	return strconv.Itoa(layer.ID) + "_" +
		strconv.Itoa(layer.Section.X) + "_" + strconv.Itoa(layer.Section.Y) + "_" +
		strconv.Itoa(layer.Section.Width) + "_" + strconv.Itoa(layer.Section.Height)
}

func (r *MaterialRenderer) drawLayer(layer textureTarget) {
	section := layer.Section
	material := layer.Material
	blendMode := layer.TextureLayer.BlendMode

	rectX := int(math.Round(float64(section.X) / float64(material.Width) * float64(r.Width)))
	rectY := int(math.Round(float64(section.Y) / float64(material.Height) * float64(r.Height)))
	rectW := int(math.Round(float64(section.Width) / float64(material.Width) * float64(r.Width)))
	rectH := int(math.Round(float64(section.Height) / float64(material.Height) * float64(r.Height)))
	if rectW <= 0 || rectH <= 0 {
		return
	}

	var baseTex *cpuTexture
	if blendMode == 4 || blendMode == 6 || blendMode == 7 {
		if material.Width == section.Width && material.Height == section.Height {
			data := make([]uint8, len(r.Canvas))
			copy(data, r.Canvas)
			baseTex = &cpuTexture{Data: data, Width: r.Width, Height: r.Height}
		} else {
			snap := make([]uint8, rectW*rectH*4)
			for fy := 0; fy < rectH; fy++ {
				srcRow := r.Height - rectY - rectH + fy
				if srcRow < 0 || srcRow >= r.Height {
					continue
				}
				for x := 0; x < rectW; x++ {
					srcCol := rectX + x
					if srcCol < 0 || srcCol >= r.Width {
						continue
					}
					src := (srcRow*r.Width + srcCol) * 4
					dst := (fy*rectW + x) * 4
					copy(snap[dst:dst+4], r.Canvas[src:src+4])
				}
			}
			baseTex = &cpuTexture{Data: snap, Width: rectW, Height: rectH}
		}
	}

	tex := layer.Texture
	texMin := math.Max(float64(tex.Width)/float64(rectW), float64(tex.Height)/float64(rectH)) > 1
	baseMin := false
	if baseTex != nil {
		baseMin = math.Max(float64(baseTex.Width)/float64(rectW), float64(baseTex.Height)/float64(rectH)) > 1
	}

	x0 := maxInt(0, rectX)
	y0 := maxInt(0, rectY)
	x1 := minInt(r.Width, rectX+rectW)
	y1 := minInt(r.Height, rectY+rectH)

	src := make([]float64, 4)
	base := make([]float64, 4)
	frag := make([]float64, 4)

	for py := y0; py < y1; py++ {
		v := (float64(py-rectY) + 0.5) / float64(rectH)
		for px := x0; px < x1; px++ {
			u := (float64(px-rectX) + 0.5) / float64(rectW)
			sampleTexture(tex, u, v, texMin, src)
			switch blendMode {
			case 0, 1, 9, 15:
				copy(frag, src)
			case 4:
				sampleTexture(*baseTex, u, v, baseMin, base)
				for c := 0; c < 4; c++ {
					frag[c] = base[c] * src[c]
				}
			case 7:
				sampleTexture(*baseTex, u, v, baseMin, base)
				for c := 0; c < 3; c++ {
					frag[c] = 1 - (1-base[c])*(1-src[c])
				}
				frag[3] = src[3]
			case 6:
				sampleTexture(*baseTex, u, v, baseMin, base)
				for c := 0; c < 3; c++ {
					if src[c] < 0.5 {
						frag[c] = 2 * base[c] * src[c]
					} else {
						frag[c] = 1 - 2*(1-base[c])*(1-src[c])
					}
				}
				frag[3] = src[3]
			default:
				frag[0], frag[1], frag[2], frag[3] = 1, 0, 1, 1
			}

			di := (py*r.Width + px) * 4
			sa := frag[3]
			if blendMode == 0 || blendMode == 1 {
				r.Canvas[di] = uint8(frag[0] * 255)
				r.Canvas[di+1] = uint8(frag[1] * 255)
				r.Canvas[di+2] = uint8(frag[2] * 255)
				r.Canvas[di+3] = uint8(sa * 255)
			} else if blendMode == 9 {
				r.Canvas[di] = uint8((frag[0]*sa + float64(r.Canvas[di])/255*(1-sa)) * 255)
				r.Canvas[di+1] = uint8((frag[1]*sa + float64(r.Canvas[di+1])/255*(1-sa)) * 255)
				r.Canvas[di+2] = uint8((frag[2]*sa + float64(r.Canvas[di+2])/255*(1-sa)) * 255)
				r.Canvas[di+3] = uint8((sa + float64(r.Canvas[di+3])/255*(1-sa)) * 255)
			} else {
				r.Canvas[di] = uint8((frag[0]*sa + float64(r.Canvas[di])/255*(1-sa)) * 255)
				r.Canvas[di+1] = uint8((frag[1]*sa + float64(r.Canvas[di+1])/255*(1-sa)) * 255)
				r.Canvas[di+2] = uint8((frag[2]*sa + float64(r.Canvas[di+2])/255*(1-sa)) * 255)
				r.Canvas[di+3] = uint8((sa*sa + float64(r.Canvas[di+3])/255*(1-sa)) * 255)
			}
		}
	}
}

func sampleTexture(tex cpuTexture, u, v float64, minified bool, out []float64) {
	if minified {
		tx := int(math.Floor(u * float64(tex.Width)))
		ty := int(math.Floor(v * float64(tex.Height)))
		tx = minInt(tex.Width-1, maxInt(0, tx))
		ty = minInt(tex.Height-1, maxInt(0, ty))
		i := (ty*tex.Width + tx) * 4
		out[0] = float64(tex.Data[i]) / 255
		out[1] = float64(tex.Data[i+1]) / 255
		out[2] = float64(tex.Data[i+2]) / 255
		out[3] = float64(tex.Data[i+3]) / 255
		return
	}
	fx := u*float64(tex.Width) - 0.5
	fy := v*float64(tex.Height) - 0.5
	x0 := int(math.Floor(fx))
	y0 := int(math.Floor(fy))
	dx := fx - float64(x0)
	dy := fy - float64(y0)
	cx0 := minInt(tex.Width-1, maxInt(0, x0))
	cx1 := minInt(tex.Width-1, maxInt(0, x0+1))
	cy0 := minInt(tex.Height-1, maxInt(0, y0))
	cy1 := minInt(tex.Height-1, maxInt(0, y0+1))
	i00 := (cy0*tex.Width + cx0) * 4
	i10 := (cy0*tex.Width + cx1) * 4
	i01 := (cy1*tex.Width + cx0) * 4
	i11 := (cy1*tex.Width + cx1) * 4
	w00 := (1 - dx) * (1 - dy)
	w10 := dx * (1 - dy)
	w01 := (1 - dx) * dy
	w11 := dx * dy
	for c := 0; c < 4; c++ {
		out[c] = (float64(tex.Data[i00+c])*w00 + float64(tex.Data[i10+c])*w10 +
			float64(tex.Data[i01+c])*w01 + float64(tex.Data[i11+c])*w11) / 255
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
