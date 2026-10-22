package terrain

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/terrainfit"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

// Match the existing, unplayable model boundary to final native ground. This
// changes no model footprint and allocates no model patches on playable cells.
func (w *Wc3Converter) FinishTilesetModels(ctx context.Context, m *common.WowObjectManager, terrain data.Terrain, fitted *terrainfit.Tileset, cfg config.Config) error {
	width, height := terrain.Map.Width, terrain.Map.Height
	ext := common.ComputeAbsoluteMinMaxExtents(m.Roots)
	size := imath.V3Sub(ext.Max, ext.Min)
	low := ext.Min[2] + size[2]*w.config.Terrain.ClampPercent.Lower
	span := size[2] * (w.config.Terrain.ClampPercent.Upper - w.config.Terrain.ClampPercent.Lower)
	for _, root := range m.Roots {
		model := w.adtModels[root]
		if model == nil {
			continue
		}
		toGrid := func(p imath.Vector3) imath.Vector3 {
			p = imath.V3Sum(root.Position, imath.V3Rotate(imath.V3Scale(p, root.ScaleFactor), root.Rotation))
			return imath.Vector3{(p[0] - ext.Min[0]) / size[0] * float64(width), (p[1] - ext.Min[1]) / size[1] * float64(height), p[2]}
		}
		// Record UV->XY before welding Z (ADT roots have only yaw).
		atlases := []*boundaryAtlas{}
		geosets := append([]*components.Geoset(nil), model.MDL.Geosets...)
		for _, g := range geosets {
			if err := ctx.Err(); err != nil {
				return err
			}
			var origin, du, dv imath.Vector3
			mapped, near := false, false
			for _, f := range g.Faces {
				a, b, c := f.Vertices[0], f.Vertices[1], f.Vertices[2]
				u1, v1 := b.TexPosition[0]-a.TexPosition[0], b.TexPosition[1]-a.TexPosition[1]
				u2, v2 := c.TexPosition[0]-a.TexPosition[0], c.TexPosition[1]-a.TexPosition[1]
				det := u1*v2 - u2*v1
				if math.Abs(det) < 1e-12 {
					continue
				}
				p, q, r := toGrid(a.Position), toGrid(b.Position), toGrid(c.Position)
				du = imath.V3Scale(imath.V3Sub(imath.V3Scale(imath.V3Sub(q, p), v2), imath.V3Scale(imath.V3Sub(r, p), v1)), 1/det)
				dv = imath.V3Scale(imath.V3Sub(imath.V3Scale(imath.V3Sub(r, p), u1), imath.V3Scale(imath.V3Sub(q, p), u2)), 1/det)
				origin = imath.V3Sub(imath.V3Sub(p, imath.V3Scale(du, a.TexPosition[0])), imath.V3Scale(dv, a.TexPosition[1]))
				mapped = true
				break
			}
			for _, v := range g.Vertices {
				p := toGrid(v.Position)
				distance, nx, ny := nearestNative(w.modelCells, p[0], p[1])
				if distance >= 1 {
					continue
				}
				near = true
				weight := boundaryWeight(distance)
				target := low + (GetTerrainHeight(terrain, nx/float64(width), ny/float64(height))-float64(config.DataHeightMin()))/float64(config.DataHeightMax()-config.DataHeightMin())*span
				v.Position[2] += (target - p[2]) * (1 - weight) / root.ScaleFactor
				// Match the native slope normal too, so the welded seam does not
				// retain an unrelated source lighting edge.
				heightAt := func(x, y float64) float64 {
					return (GetTerrainHeight(terrain, x/float64(width), y/float64(height)) - float64(config.DataHeightMin())) / float64(config.DataHeightMax()-config.DataHeightMin()) * span
				}
				dx := (heightAt(nx+.5, ny) - heightAt(nx-.5, ny)) / (size[0] / float64(width))
				dy := (heightAt(nx, ny+.5) - heightAt(nx, ny-.5)) / (size[1] / float64(height))
				normal := imath.V3Rotate(imath.V3Normalize(imath.Vector3{-dx, -dy, 1}), imath.EulerRotation{0, 0, -root.Rotation[2]})
				v.Normal = imath.V3Normalize(imath.V3Sum(imath.V3Scale(v.Normal, weight), imath.V3Scale(normal, 1-weight)))
			}
			if !near || !mapped || g.Material == nil || len(g.Material.Layers) != 1 {
				continue
			}
			texture := g.Material.Layers[0].Texture
			if texture == nil {
				continue
			}
			path := texture.WowData.PngPath
			var raw []byte
			if source, ok := texturesource.Get(path); ok && source.Kind == texturesource.KindPNG {
				raw = source.PNG
			} else {
				var err error
				raw, err = os.ReadFile(filepath.Join(cfg.ExportAssetDir, path))
				if err != nil {
					return fmt.Errorf("read boundary texture %s: %w", path, err)
				}
			}
			source, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				return err
			}
			img := image.NewNRGBA(source.Bounds())
			draw.Draw(img, img.Bounds(), source, source.Bounds().Min, draw.Src)
			patches := map[[2]int]*boundaryPatch{}
			patched := map[*boundaryAtlas]*components.Geoset{}
			interior := make([]components.Face, 0, len(g.Faces))
			for _, face := range g.Faces {
				p := imath.Vector3{}
				for _, v := range face.Vertices {
					p = imath.V3Sum(p, toGrid(v.Position))
				}
				p = imath.V3Scale(p, 1.0/3)
				x, y := int(math.Floor(p[0])), int(math.Floor(p[1]))
				// Faces were already clipped to individual native cells. Only the
				// one-cell model-side transition needs native-density texture data.
				d, _, _ := nearestNative(w.modelCells, float64(x)+.5, float64(y)+.5)
				if d >= 1 {
					interior = append(interior, face)
					continue
				}
				key := [2]int{x, y}
				patch := patches[key]
				if patch == nil {
					if len(atlases) == 0 || atlases[len(atlases)-1].used == boundaryAtlasCapacity {
						tex := *texture
						rel := strings.TrimPrefix(model.RelativePath, strings.TrimRight(cfg.AssetPrefix, "/\\")+"/") + fmt.Sprintf("-edge-atlas-%02d.png", len(atlases))
						tex.WowData.PngPath = rel
						tex.Image = filepath.ToSlash(filepath.Join(cfg.AssetPrefix, strings.TrimSuffix(rel, ".png")+".blp"))
						atlases = append(atlases, &boundaryAtlas{image: image.NewNRGBA(image.Rect(0, 0, boundaryAtlasSize, boundaryAtlasSize)), texture: &tex})
						model.MDL.Textures = append(append([]*components.Texture(nil), model.MDL.Textures...), &tex)
					}
					atlas := atlases[len(atlases)-1]
					patch = &boundaryPatch{atlas: atlas, x: atlas.used % boundaryAtlasColumns * boundaryPatchSize, y: atlas.used / boundaryAtlasColumns * boundaryPatchSize, vertices: map[*components.GeosetVertex]*components.GeosetVertex{}}
					atlas.used++
					patches[key] = patch
					if err := bakeBoundaryPatch(ctx, patch, x, y, img, origin, du, dv, w.modelCells, fitted); err != nil {
						return err
					}
				}
				out := patched[patch.atlas]
				if out == nil {
					cp := *g
					cp.Vertices = nil
					cp.Faces = nil
					mat := *g.Material
					mat.Layers = append([]components.Layer(nil), mat.Layers...)
					mat.Layers[0].Texture = patch.atlas.texture
					cp.Material = &mat
					out = &cp
					patched[patch.atlas] = out
					model.MDL.Materials = append(append([]*components.Material(nil), model.MDL.Materials...), &mat)
					model.MDL.Geosets = append(model.MDL.Geosets, out)
				}
				f := components.Face{}
				for k, v := range face.Vertices {
					vv := patch.vertices[v]
					if vv == nil {
						cp := *v
						pos := toGrid(v.Position)
						cp.TexPosition = imath.Vector2{(float64(patch.x+boundaryGutter) + (pos[0]-float64(x))*boundaryTileSize) / boundaryAtlasSize, (float64(patch.y+boundaryGutter) + (float64(y+1)-pos[1])*boundaryTileSize) / boundaryAtlasSize}
						vv = &cp
						patch.vertices[v] = vv
						out.Vertices = append(out.Vertices, vv)
					}
					f.Vertices[k] = vv
				}
				out.Faces = append(out.Faces, f)
			}
			g.Faces = interior
			used := map[*components.GeosetVertex]bool{}
			for _, f := range interior {
				for _, v := range f.Vertices {
					used[v] = true
				}
			}
			vertices := make([]*components.GeosetVertex, 0, len(g.Vertices))
			for _, v := range g.Vertices {
				if used[v] {
					vertices = append(vertices, v)
				}
			}
			g.Vertices = vertices
		}
		for _, atlas := range atlases {
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, atlas.image); err != nil {
				return err
			}
			rel := atlas.texture.WowData.PngPath
			texturesource.Register(rel, texturesource.Source{Kind: texturesource.KindPNG, PNG: encoded.Bytes(), Opaque: true})
			m.AssetManager.AddPngTexture(rel, true)
		}
		nonempty := make([]*components.Geoset, 0, len(model.MDL.Geosets))
		for _, g := range model.MDL.Geosets {
			if len(g.Faces) > 0 {
				nonempty = append(nonempty, g)
			}
		}
		model.MDL.Geosets = nonempty
		model.MDL.Modify.RemoveUnusedMaterialsTextures()
	}
	return nil
}

func boundaryWeight(distance float64) float64 {
	t := math.Max(0, math.Min(1, distance))
	return t * t * (3 - 2*t)
}

// Return a point just inside nearby native ground; sampling the model-side
// alpha tile itself would bake a black line into the transition texture.
func nearestNative(model [][]bool, x, y float64) (distance, nx, ny float64) {
	distance = 1
	nx, ny = x, y
	height, width := len(model), len(model[0])
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	for yy := max(0, iy-1); yy <= min(height-1, iy+1); yy++ {
		for xx := max(0, ix-1); xx <= min(width-1, ix+1); xx++ {
			if model[yy][xx] {
				continue
			}
			px := math.Max(float64(xx)+1e-6, math.Min(float64(xx+1)-1e-6, x))
			py := math.Max(float64(yy)+1e-6, math.Min(float64(yy+1)-1e-6, y))
			d := math.Hypot(px-x, py-y)
			if d < distance {
				distance, nx, ny = d, px, py
			}
		}
	}
	return
}

// Match native 64px/cell density. Gutters retain bilinear/mipmap continuity
// without storing a high-resolution copy of every mostly unchanged MCNK.
const (
	boundaryTileSize      = 64
	boundaryGutter        = 8
	boundaryPatchSize     = boundaryTileSize + 2*boundaryGutter
	boundaryAtlasSize     = 1024
	boundaryAtlasColumns  = boundaryAtlasSize / boundaryPatchSize
	boundaryAtlasCapacity = boundaryAtlasColumns * boundaryAtlasColumns
)

type boundaryAtlas struct {
	image   *image.NRGBA
	texture *components.Texture
	used    int
}
type boundaryPatch struct {
	atlas    *boundaryAtlas
	x, y     int
	vertices map[*components.GeosetVertex]*components.GeosetVertex
}

func bakeBoundaryPatch(ctx context.Context, patch *boundaryPatch, cellX, cellY int, source *image.NRGBA, origin, du, dv imath.Vector3, model [][]bool, fitted *terrainfit.Tileset) error {
	det := du[0]*dv[1] - du[1]*dv[0]
	if math.Abs(det) < 1e-12 {
		return fmt.Errorf("degenerate boundary UV mapping")
	}
	for y := 0; y < boundaryPatchSize; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		for x := 0; x < boundaryPatchSize; x++ {
			gx := float64(cellX) + (float64(x-boundaryGutter)+.5)/boundaryTileSize
			gy := float64(cellY+1) - (float64(y-boundaryGutter)+.5)/boundaryTileSize
			px, py := gx-origin[0], gy-origin[1]
			u, v := (px*dv[1]-py*dv[0])/det, (du[0]*py-du[1]*px)/det
			color := sampleBoundarySource(source, u, v)
			d, _, _ := nearestNative(model, gx, gy)
			weight := boundaryWeight(d)
			native := fitted.Sample(gx, float64(fitted.Height)-gy)
			at := (patch.y+y)*patch.atlas.image.Stride + (patch.x+x)*4
			for k := 0; k < 3; k++ {
				patch.atlas.image.Pix[at+k] = byte(math.Max(0, math.Min(255, color[k]*weight+native[k]*(1-weight))) + .5)
			}
			patch.atlas.image.Pix[at+3] = 255
		}
	}
	return nil
}

func sampleBoundarySource(img *image.NRGBA, u, v float64) [3]float64 {
	x := math.Max(0, math.Min(float64(img.Rect.Dx()-1), u*float64(img.Rect.Dx())-.5))
	y := math.Max(0, math.Min(float64(img.Rect.Dy()-1), v*float64(img.Rect.Dy())-.5))
	ix, iy := int(x), int(y)
	fx, fy := x-float64(ix), y-float64(iy)
	var out [3]float64
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			weight := []float64{1 - fx, fx}[i] * []float64{1 - fy, fy}[j]
			at := min(iy+j, img.Rect.Dy()-1)*img.Stride + min(ix+i, img.Rect.Dx()-1)*4
			for k := 0; k < 3; k++ {
				out[k] += float64(img.Pix[at+k]) * weight
			}
		}
	}
	return out
}
