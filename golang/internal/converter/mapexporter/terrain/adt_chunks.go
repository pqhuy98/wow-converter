package terrain

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

// Change this to 8 for 64 models per ADT. Empty parts are omitted.
const adtChunksPerSide = 4

type adtChunk struct {
	model  *common.Model
	center imath.Vector3
}

// PrepareAdtChunks keeps source roots intact for terrain, water and navigation.
// Only the visual models/placements are partitioned, after tileset boundary baking.
func (w *Wc3Converter) PrepareAdtChunks(ctx context.Context, manager *common.WowObjectManager, assetPrefix string) error {
	w.adtChunks = map[*common.WowObject][]adtChunk{}
	// Smooth the original full tiles before cloning their normals into parts.
	manager.AssetManager.SmoothAdtTileBorders()
	for _, root := range manager.Roots {
		if err := ctx.Err(); err != nil {
			return err
		}
		if root.Type != common.WowObjectADT || root.Model == nil {
			continue
		}
		visual := root.Model
		if w.config.TerrainAsWc3Tilesets {
			visual = w.adtModels[root]
		}
		w.adtChunks[root] = nil
		if visual == nil {
			continue
		}
		chunks := splitAdtModel(root.Model, visual, adtChunksPerSide)
		w.adtChunks[root] = chunks
		for _, chunk := range chunks {
			key := strings.TrimPrefix(chunk.model.RelativePath, strings.TrimRight(assetPrefix, "/\\")+"/")
			manager.AssetManager.Models()[key] = chunk.model
		}
	}
	return nil
}

func splitAdtModel(source, visual *common.Model, perSide int) []adtChunk {
	// Partition in source-local space. Full-model paths and geometry never use
	// the map's clamp, angle or dimensions.
	min, max := adtVertexBounds(source.MDL)
	parts := make([][]*components.Geoset, perSide*perSide)
	for _, g := range visual.MDL.Geosets {
		byPart := make([]*components.Geoset, len(parts))
		vertices := make([]map[*components.GeosetVertex]*components.GeosetVertex, len(parts))
		for _, face := range g.Faces {
			centroid := imath.Vector3{}
			for _, v := range face.Vertices {
				centroid = imath.V3Sum(centroid, imath.V3Scale(v.Position, 1.0/3))
			}
			x, y := 0, 0
			if max[0] > min[0] {
				x = int(math.Floor((centroid[0] - min[0]) / (max[0] - min[0]) * float64(perSide)))
			}
			if max[1] > min[1] {
				y = int(math.Floor((centroid[1] - min[1]) / (max[1] - min[1]) * float64(perSide)))
			}
			index := maxInt(0, minInt(perSide-1, y))*perSide + maxInt(0, minInt(perSide-1, x))
			out := byPart[index]
			if out == nil {
				copy := *g
				copy.Vertices, copy.Faces = nil, nil
				out = &copy
				byPart[index] = out
				parts[index] = append(parts[index], out)
				vertices[index] = map[*components.GeosetVertex]*components.GeosetVertex{}
			}
			f := components.Face{}
			for i, vertex := range face.Vertices {
				v := vertices[index][vertex]
				if v == nil {
					copy := *vertex
					v = &copy
					vertices[index][vertex] = v
					out.Vertices = append(out.Vertices, v)
				}
				f.Vertices[i] = v
			}
			out.Faces = append(out.Faces, f)
		}
	}
	var chunks []adtChunk
	for index, geosets := range parts {
		if len(geosets) == 0 {
			continue
		}
		fork := mdl.ForkCollectionModel(mdl.CollectionModel{MDL: visual.MDL}, geosets).MDL
		// Collection forks intentionally share bones. Terrain parts need owned
		// bones/matrices so parallel serialization and centering stay isolated.
		cloneAdtBones(fork)
		fork.CollisionShapes = nil // The map's terrain/pathing owns ground collision.
		fork.Modify.RemoveUnusedMaterialsTextures()
		lo, hi := adtVertexBounds(fork)
		center := imath.V3Mean(lo, hi)
		fork.Modify.Translate(imath.V3Negative(center))
		// SyncExtents measures model, sequences and geosets from the new origin.
		// Preserve a small rounding margin when extents become float32 in MDX.
		fork.BoundsOverriden = func(b *components.Bound) { b.BoundsRadius += math.Max(1e-4, b.BoundsRadius*1e-6) }
		stem := strings.TrimSuffix(strings.TrimSuffix(visual.RelativePath, ".mdx"), ".mdl")
		name := fmt.Sprintf("%s-chunks-v1-%dx%d-%02d-%02d", stem, perSide, perSide, index%perSide, index/perSide)
		fork.Model.Name = name
		chunks = append(chunks, adtChunk{center: center, model: &common.Model{RelativePath: name, MDL: fork, AlwaysExport: true, TightBounds: true}})
	}
	return chunks
}

func adtVertexBounds(m *mdl.MDL) (lo, hi imath.Vector3) {
	lo, hi = imath.V3All(math.Inf(1)), imath.V3All(math.Inf(-1))
	for _, g := range m.Geosets {
		for _, v := range g.Vertices {
			lo, hi = imath.V3Min(lo, v.Position), imath.V3Max(hi, v.Position)
		}
	}
	return
}

func cloneAdtBones(m *mdl.MDL) {
	bones := map[*components.Bone]*components.Bone{}
	m.Bones = append([]*components.Bone(nil), m.Bones...)
	for i, source := range m.Bones {
		copy := *source
		for _, track := range []**components.Animation{&copy.Translation, &copy.Rotation, &copy.Scaling} {
			if *track != nil {
				anim := **track
				anim.KeyFrames = components.CloneKeyFrames(anim.KeyFrames)
				*track = &anim
			}
		}
		bones[source], m.Bones[i] = &copy, &copy
	}
	for _, b := range m.Bones {
		if parent, ok := b.Parent.(*components.Bone); ok {
			b.Parent = bones[parent]
		}
		b.ParentBone = bones[b.ParentBone]
	}
	for _, g := range m.Geosets {
		matrices := map[*components.Matrix]*components.Matrix{}
		for i := range g.Matrices {
			matrix := &g.Matrices[i]
			for j, b := range matrix.Bones {
				matrix.Bones[j] = bones[b]
			}
		}
		for _, v := range g.Vertices {
			if v.Matrix != nil {
				if matrices[v.Matrix] == nil {
					copy := *v.Matrix
					copy.Bones = make([]*components.Bone, len(v.Matrix.Bones))
					for i, b := range v.Matrix.Bones {
						copy.Bones[i] = bones[b]
					}
					matrices[v.Matrix] = &copy
				}
				v.Matrix = matrices[v.Matrix]
			}
			v.SkinWeights = append([]components.SkinWeight(nil), v.SkinWeights...)
			for i := range v.SkinWeights {
				v.SkinWeights[i].Bone = bones[v.SkinWeights[i].Bone]
			}
		}
	}
}
