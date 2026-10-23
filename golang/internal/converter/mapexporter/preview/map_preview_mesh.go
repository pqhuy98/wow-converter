package preview

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/formats/png"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/liquid"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

type previewFloor struct {
	V [3]imath.Vector3
}
type previewWMO struct {
	Asset      PreviewAsset
	FloorFile  string
	FloorCount int
	Floors     []previewFloor `json:"-"`
}

func previewVertexBytes(vertices []float32) []byte {
	out := make([]byte, len(vertices)*4)
	for i, v := range vertices {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(v))
	}
	return out
}

func previewADTVertex(ch adt.ADTChunk, row, x int) imath.Vector3 {
	column := float64(x)
	if row%2 != 0 {
		column += .5
	}
	z := ch.Vertices[row/2*17+(row%2)*9+x]
	return imath.Vector3{-float64(ch.Position[1]) + column*533.3333333333333/128, float64(ch.Position[0]) - float64(row)*533.3333333333333/256, float64(ch.Position[2]) + float64(z)}
}

func previewHole(ch previewChunk, x, y int) bool {
	if ch.Flags&0x10000 != 0 {
		return y < len(ch.HighHoles) && ch.HighHoles[y]&(1<<uint(x)) != 0
	}
	return ch.Holes&(1<<uint(x/2+(y/2)*4)) != 0
}

// The exporter drops hole faces before assembling the terrain. Unused vertices
// must not widen the height range and allow an unsafe minimum slider gap.
func previewTerrainVertices(ch previewChunk) (used [145]bool) {
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if previewHole(ch, x, y) {
				continue
			}
			center := y*17 + 9 + x
			for _, i := range []int{center, center - 9, center - 8, center + 8, center + 9} {
				used[i] = true
			}
		}
	}
	return used
}

func previewTerrainMesh(t previewTile) []float32 {
	var out []float32
	for _, ch := range t.Chunks {
		step := 2
		hasHoles := ch.Holes != 0
		if ch.Flags&0x10000 != 0 {
			hasHoles = false
			for _, holes := range ch.HighHoles {
				hasHoles = hasHoles || holes != 0
			}
		}
		if hasHoles {
			step = 1
		}
		for y := 0; y < 8; y += step {
			for x := 0; x < 8; x += step {
				if previewHole(ch, x, y) {
					continue
				}
				var p [4]imath.Vector3
				for i, d := range [][2]int{{0, 0}, {step, 0}, {step, step}, {0, step}} {
					xx, yy := x+d[0], y+d[1]
					p[i] = imath.Vector3{-float64(ch.Position[1]) + float64(xx)*533.3333333333333/128, float64(ch.Position[0]) - float64(yy)*533.3333333333333/128, float64(ch.Position[2]) + float64(ch.Heights[yy*17+xx])}
				}
				for _, ids := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
					v := [3]imath.Vector3{p[ids[0]], p[ids[1]], p[ids[2]]}
					n := imath.V3Normalize(mapterrain.NavigationCross(imath.V3Sub(v[1], v[0]), imath.V3Sub(v[2], v[0])))
					if n[2] < 0 {
						n = imath.V3Negative(n)
					}
					for _, pt := range v {
						u := (pt[0] - t.Min[0]) / (t.Max[0] - t.Min[0])
						vv := (t.Max[1] - pt[1]) / (t.Max[1] - t.Min[1])
						pt = imath.V3Sub(pt, t.Origin)
						out = append(out, float32(pt[0]), float32(pt[1]), float32(pt[2]), float32(n[0]), float32(n[1]), float32(n[2]), float32(u), float32(vv))
					}
				}
			}
		}
	}
	return out
}

func (b *previewBuilder) texture(id uint32, name, path string) (string, error) {
	if _, err := os.Stat(b.full(path)); err == nil {
		return path, nil
	}
	raw, err := b.file(id, name)
	if err != nil {
		return "", err
	}
	image, err := blp.NewBLPImage(buffer.From(raw))
	if err != nil {
		return "", err
	}
	if image.MapCount == 0 || image.Width == 0 || image.Height == 0 {
		return "", fmt.Errorf("texture contains no mipmaps")
	}
	mip := 0
	for mip+1 < image.MapCount && max(image.Width, image.Height)>>uint(mip) > 512 {
		mip++
	}
	encoded, err := image.ToPNG(15, mip)
	if err != nil {
		return "", err
	}
	if max(image.Width, image.Height)>>uint(mip) > 512 {
		scale := 512 / float64(max(image.Width, image.Height)>>uint(mip))
		resized, err := png.ResizePngBytes(encoded.Raw(), max(1, int(float64(image.ScaledWidth)*scale)), max(1, int(float64(image.ScaledHeight)*scale)))
		if err != nil {
			return "", err
		}
		encoded = buffer.From(resized)
	}
	if err = b.write(path, encoded.Raw()); err != nil {
		return "", err
	}
	return path, nil
}

func (b *previewBuilder) wmoRoot(id int) (*wmo.Loader, error) {
	if root := b.wmoRoots[id]; root != nil {
		return root, nil
	}
	entry, err := b.options.Client.GetFileByID(b.ctx, id)
	if err != nil {
		return nil, err
	}
	raw, err := b.file(uint32(id), "")
	if err != nil {
		return nil, err
	}
	root := wmo.NewLoader(buffer.From(raw), id, entry.FileName, true)
	if err := root.Load(); err != nil {
		return nil, err
	}
	if b.wmoRoots == nil {
		b.wmoRoots = map[int]*wmo.Loader{}
	}
	b.wmoRoots[id] = root
	return root, nil
}

func (b *previewBuilder) wmo(id int) (previewWMO, error) {
	stem := fmt.Sprintf("%s/wmo-%d", b.prefix, id)
	var result previewWMO
	err := b.cached(stem+".json", &result, func() error {
		result = previewWMO{}
		root, err := b.wmoRoot(id)
		if err != nil {
			return err
		}
		result.Asset = PreviewAsset{ID: stem, Parts: []PreviewPart{}}
		// Group all batches sharing a thumbnail texture into one draw/upload.
		partIndices := map[string]int{}
		var partVertices [][]float32
		var liquids []liquid.Surface
		rootCell := .02
		if len(root.BoundingBox1) == 3 && len(root.BoundingBox2) == 3 {
			dx, dy, dz := float64(root.BoundingBox2[0]-root.BoundingBox1[0]), float64(root.BoundingBox2[1]-root.BoundingBox1[1]), float64(root.BoundingBox2[2]-root.BoundingBox1[2])
			rootCell = max(rootCell, math.Sqrt(dx*dx+dy*dy+dz*dz)/128)
		}
		for groupIndex := 0; groupIndex < int(root.GroupCount); groupIndex++ {
			if err := b.progress("Preparing WMO groups", b.wmoGroupsCompleted+groupIndex, b.wmoGroupsTotal); err != nil {
				return err
			}
			groupID := uint32(0)
			if groupIndex < len(root.GroupIDs) {
				groupID = root.GroupIDs[groupIndex]
			}
			name := fmt.Sprintf("%s_%03d.wmo", strings.TrimSuffix(root.FileName, ".wmo"), groupIndex)
			raw, err := b.file(groupID, name)
			if err != nil {
				return err
			}
			g := wmo.NewLoader(buffer.From(raw), int(groupID), name, true)
			g.IncludeLiquid = true
			if err = g.Load(); err != nil {
				return err
			}
			surfaces, err := g.LiquidSurfaces(root.Flags)
			if err != nil {
				return err
			}
			// WMO liquids already use native local XYZ, like our display mesh;
			// the scene instance applies the placement transform exactly once.
			liquids = append(liquids, surfaces...)
			point := func(index int) (imath.Vector3, error) {
				i := index * 3
				if i+2 >= len(g.Vertices) {
					return imath.Vector3{}, fmt.Errorf("WMO vertex index outside buffer")
				}
				return imath.Vector3{float64(g.Vertices[i]), -float64(g.Vertices[i+2]), float64(g.Vertices[i+1])}, nil
			}
			// Full source collision floors are cheap metadata; display LOD never
			// determines whether a marker is standing on a WMO road/bridge.
			for face, info := range g.MaterialInfo {
				if info.Flags&8 == 0 && !(info.Flags&0x20 != 0 && info.Flags&4 == 0) && info.MaterialID != 255 {
					continue
				}
				if face*3+2 >= len(g.Indices) {
					return fmt.Errorf("WMO collision face outside buffer")
				}
				var v [3]imath.Vector3
				for k := range v {
					p, err := point(int(g.Indices[face*3+k]))
					if err != nil {
						return err
					}
					v[k] = p
				}
				result.Floors = append(result.Floors, previewFloor{V: v})
			}
			lo, hi := imath.V3All(math.Inf(1)), imath.V3All(math.Inf(-1))
			for i := 0; i < len(g.Vertices)/3; i++ {
				p, err := point(i)
				if err != nil {
					return err
				}
				lo = imath.V3Min(lo, p)
				hi = imath.V3Max(hi, p)
			}
			// ponytail: linear-time vertex clustering removes sub-thumbnail detail;
			// it is a static overview LOD, not topology-preserving simplification.
			cell := max(rootCell, imath.V3Distance(lo, hi)/96)
			for _, batch := range g.RenderBatches {
				material := int(batch.MaterialID)
				if batch.Flags&2 != 0 && len(batch.PossibleBox2) > 2 {
					material = int(batch.PossibleBox2[2])
				}
				part := PreviewPart{}
				var vertices []float32
				seenFaces := map[[9]float64]bool{}
				for i := int(batch.FirstFace); i < int(batch.FirstFace)+int(batch.NumFaces); i += 3 {
					if i+2 >= len(g.Indices) {
						return fmt.Errorf("WMO render face outside buffer")
					}
					var p [3]imath.Vector3
					for k := range p {
						pt, err := point(int(g.Indices[i+k]))
						if err != nil {
							return err
						}
						for axis := range pt {
							pt[axis] = math.Round(pt[axis]/cell) * cell
						}
						p[k] = pt
					}
					n := imath.V3Normalize(mapterrain.NavigationCross(imath.V3Sub(p[1], p[0]), imath.V3Sub(p[2], p[0])))
					if imath.V3Dot(n, n) == 0 {
						continue
					}
					key := [9]float64{p[0][0], p[0][1], p[0][2], p[1][0], p[1][1], p[1][2], p[2][0], p[2][1], p[2][2]}
					if seenFaces[key] {
						continue
					}
					seenFaces[key] = true
					for k, pt := range p {
						u, v := float32(0), float32(0)
						index := int(g.Indices[i+k]) * 2
						if len(g.UVs) > 0 && index+1 < len(g.UVs[0]) {
							u, v = g.UVs[0][index], 1-g.UVs[0][index+1]
						}
						vertices = append(vertices, float32(pt[0]), float32(pt[1]), float32(pt[2]), float32(n[0]), float32(n[1]), float32(n[2]), u, v)
					}
				}
				if len(vertices) == 0 {
					continue
				}
				if material < len(root.Materials) {
					texture := root.Materials[material].Texture1
					textureName := ""
					if len(root.TextureNames) > 0 {
						textureName = root.TextureNames[int(texture)]
						texture = 0
					}
					texturePath := fmt.Sprintf("%s/tex-%s.png", b.prefix, previewHash(fmt.Sprintf("%d:%s", texture, textureName)))
					if path, e := b.texture(texture, textureName, texturePath); e == nil {
						part.Texture = path
					}
				}
				index, found := partIndices[part.Texture]
				if !found {
					index = len(result.Asset.Parts)
					partIndices[part.Texture] = index
					part.Mesh = fmt.Sprintf("%s-p%d.bin", stem, index)
					result.Asset.Parts = append(result.Asset.Parts, part)
					partVertices = append(partVertices, nil)
				}
				partVertices[index] = append(partVertices[index], vertices...)
			}
		}
		if err := b.progress("Preparing WMO groups", b.wmoGroupsCompleted+int(root.GroupCount), b.wmoGroupsTotal); err != nil {
			return err
		}
		for i, vertices := range partVertices {
			result.Asset.Parts[i].Vertices = len(vertices) / 8
			if err := b.write(result.Asset.Parts[i].Mesh, previewVertexBytes(vertices)); err != nil {
				return err
			}
		}
		if err := b.addLiquids(&result.Asset, liquids); err != nil {
			return err
		}
		result.FloorCount = len(result.Floors)
		result.FloorFile = stem + "-floors.bin"
		rawFloors := make([]byte, result.FloorCount*36)
		for i, f := range result.Floors {
			for v, p := range f.V {
				for axis, n := range p {
					binary.LittleEndian.PutUint32(rawFloors[i*36+v*12+axis*4:], math.Float32bits(float32(n)))
				}
			}
		}
		if err := b.write(result.FloorFile, rawFloors); err != nil {
			return err
		}
		return nil
	})
	if err == nil && len(result.Floors) == 0 && result.FloorCount > 0 {
		var raw []byte
		raw, err = os.ReadFile(b.full(result.FloorFile))
		if err == nil && len(raw) != result.FloorCount*36 {
			err = fmt.Errorf("incomplete WMO collision thumbnail metadata")
		}
		if err == nil {
			result.Floors = make([]previewFloor, result.FloorCount)
			for i := range result.Floors {
				for v := range result.Floors[i].V {
					for axis := range result.Floors[i].V[v] {
						result.Floors[i].V[v][axis] = float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[i*36+v*12+axis*4:])))
					}
				}
			}
		}
	}
	return result, err
}
