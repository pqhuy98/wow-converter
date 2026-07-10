package mdl

import (
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

type clipPlane struct {
	axis int
	min  bool
	value float64
}

func (mod *Modify) DeleteVerticesIf(shouldDeleteVert func(v *components.GeosetVertex) bool, resolvePartialFace func(f components.Face) []components.Face) *Modify {
	for _, geoset := range mod.MDL.Geosets {
		verts := map[*components.GeosetVertex]struct{}{}
		for _, v := range geoset.Vertices {
			verts[v] = struct{}{}
		}
		faces := map[components.Face]struct{}{}
		for _, face := range geoset.Faces {
			faces[face] = struct{}{}
		}
		for _, vert := range geoset.Vertices {
			if shouldDeleteVert != nil && shouldDeleteVert(vert) {
				delete(verts, vert)
			}
		}
		for face := range faces {
			needsSplit := false
			for _, v := range face.Vertices {
				if _, ok := verts[v]; !ok {
					needsSplit = true
					break
				}
			}
			if needsSplit {
				delete(faces, face)
				if resolvePartialFace != nil {
					for _, newFace := range resolvePartialFace(face) {
						faces[newFace] = struct{}{}
						for _, v := range newFace.Vertices {
							verts[v] = struct{}{}
						}
					}
				}
			}
		}
		verts = map[*components.GeosetVertex]struct{}{}
		for face := range faces {
			for _, v := range face.Vertices {
				verts[v] = struct{}{}
			}
		}
		var newVerts []*components.GeosetVertex
		for v := range verts {
			newVerts = append(newVerts, v)
		}
		geoset.Vertices = newVerts
		geoset.Faces = keysFaces(faces)
	}
	return mod
}

func keysFaces(m map[components.Face]struct{}) []components.Face {
	out := make([]components.Face, 0, len(m))
	for f := range m {
		out = append(out, f)
	}
	return out
}

func (mod *Modify) DeleteVerticesOutsideBox(low, high imath.Vector3) *Modify {
	shouldDeleteVert := func(vert *components.GeosetVertex) bool {
		return vert.Position[0] < low[0] || vert.Position[1] < low[1] || vert.Position[2] < low[2] ||
			vert.Position[0] > high[0] || vert.Position[1] > high[1] || vert.Position[2] > high[2]
	}
	resolvePartialFace := func(face components.Face) []components.Face {
		planes := []clipPlane{
			{0, true, low[0]}, {0, false, high[0]},
			{1, true, low[1]}, {1, false, high[1]},
			{2, true, low[2]}, {2, false, high[2]},
		}
		poly := []*components.GeosetVertex{face.Vertices[0], face.Vertices[1], face.Vertices[2]}
		for _, plane := range planes {
			poly = clipPolygon(poly, plane)
			if len(poly) == 0 {
				return nil
			}
		}
		var outFaces []components.Face
		for i := 1; i < len(poly)-1; i++ {
			outFaces = append(outFaces, components.Face{Vertices: [3]*components.GeosetVertex{poly[0], poly[i], poly[i+1]}})
		}
		return outFaces
	}
	mod.DeleteVerticesIf(shouldDeleteVert, resolvePartialFace)
	return mod
}

func (mod *Modify) DeleteVerticesInsideBox(low, high imath.Vector3) *Modify {
	shouldDeleteVert := func(vert *components.GeosetVertex) bool {
		return vert.Position[0] >= low[0] && vert.Position[1] >= low[1] && vert.Position[2] >= low[2] &&
			vert.Position[0] <= high[0] && vert.Position[1] <= high[1] && vert.Position[2] <= high[2]
	}
	resolvePartialFace := func(face components.Face) []components.Face {
		planes := []clipPlane{
			{0, false, low[0]}, {0, true, high[0]},
			{1, false, low[1]}, {1, true, high[1]},
			{2, false, low[2]}, {2, true, high[2]},
		}
		var outFaces []components.Face
		for _, plane := range planes {
			poly := clipPolygon([]*components.GeosetVertex{face.Vertices[0], face.Vertices[1], face.Vertices[2]}, plane)
			if len(poly) < 3 {
				continue
			}
			for i := 1; i < len(poly)-1; i++ {
				outFaces = append(outFaces, components.Face{Vertices: [3]*components.GeosetVertex{poly[0], poly[i], poly[i+1]}})
			}
		}
		return outFaces
	}
	mod.DeleteVerticesIf(shouldDeleteVert, resolvePartialFace)
	return mod
}

func (mod *Modify) Cut1DimOutside(dimension int, lowPercent, highPercent float64) *Modify {
	diff := mod.MDL.Model.MaximumExtent[dimension] - mod.MDL.Model.MinimumExtent[dimension]
	low := mod.MDL.Model.MinimumExtent[dimension] + diff*lowPercent
	high := mod.MDL.Model.MinimumExtent[dimension] + diff*highPercent
	vLow := imath.Vector3{-1e100, -1e100, -1e100}
	vHigh := imath.Vector3{1e100, 1e100, 1e100}
	vLow[dimension] = low
	vHigh[dimension] = high
	return mod.DeleteVerticesOutsideBox(vLow, vHigh)
}

func (mod *Modify) CutInsidePercent(bounds [3][2]float64) *Modify {
	vLow := imath.Vector3{
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[0], mod.MDL.Model.MaximumExtent[0], bounds[0][0]),
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[1], mod.MDL.Model.MaximumExtent[1], bounds[1][0]),
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[2], mod.MDL.Model.MaximumExtent[2], bounds[2][0]),
	}
	vHigh := imath.Vector3{
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[0], mod.MDL.Model.MaximumExtent[0], bounds[0][1]),
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[1], mod.MDL.Model.MaximumExtent[1], bounds[1][1]),
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[2], mod.MDL.Model.MaximumExtent[2], bounds[2][1]),
	}
	return mod.DeleteVerticesInsideBox(vLow, vHigh)
}

func (mod *Modify) CutOutsidePercent(bounds [3][2]float64) *Modify {
	mod.MDL.Sync()
	vLow := imath.Vector3{
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[0], mod.MDL.Model.MaximumExtent[0], bounds[0][0]),
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[1], mod.MDL.Model.MaximumExtent[1], bounds[1][0]),
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[2], mod.MDL.Model.MaximumExtent[2], bounds[2][0]),
	}
	vHigh := imath.Vector3{
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[0], mod.MDL.Model.MaximumExtent[0], bounds[0][1]),
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[1], mod.MDL.Model.MaximumExtent[1], bounds[1][1]),
		imath.V3LerpScalar(mod.MDL.Model.MinimumExtent[2], mod.MDL.Model.MaximumExtent[2], bounds[2][1]),
	}
	return mod.DeleteVerticesOutsideBox(vLow, vHigh)
}

func (mod *Modify) CropVerticesOneDimension(dimension int, low, high float64) *Modify {
	mod.MDL.Sync()
	vLow := imath.Vector3{-1e100, -1e100, -1e100}
	vHigh := imath.Vector3{1e100, 1e100, 1e100}
	vLow[dimension] = low
	vHigh[dimension] = high
	return mod.DeleteVerticesOutsideBox(vLow, vHigh)
}

func (mod *Modify) DeleteFacesIf(shouldDeleteFace func(face components.Face, geoset *components.Geoset) bool) *Modify {
	for _, geoset := range mod.MDL.Geosets {
		verts := map[*components.GeosetVertex]struct{}{}
		for _, v := range geoset.Vertices {
			verts[v] = struct{}{}
		}
		faces := map[components.Face]struct{}{}
		for _, face := range geoset.Faces {
			faces[face] = struct{}{}
		}
		for face := range faces {
			if shouldDeleteFace(face, geoset) {
				delete(faces, face)
			}
		}
		verts = map[*components.GeosetVertex]struct{}{}
		for face := range faces {
			for _, v := range face.Vertices {
				verts[v] = struct{}{}
			}
		}
		var newVerts []*components.GeosetVertex
		for v := range verts {
			newVerts = append(newVerts, v)
		}
		geoset.Vertices = newVerts
		geoset.Faces = keysFaces(faces)
	}
	return mod
}

func interpolateVertex(v1, v2 *components.GeosetVertex, t float64) *components.GeosetVertex {
	lerp := func(a, b float64) float64 { return a + (b-a)*t }
	interpVec3 := func(a, b imath.Vector3) imath.Vector3 {
		return imath.Vector3{lerp(a[0], b[0]), lerp(a[1], b[1]), lerp(a[2], b[2])}
	}
	interpVec2 := func(a, b imath.Vector2) imath.Vector2 {
		return imath.Vector2{lerp(a[0], b[0]), lerp(a[1], b[1])}
	}

	var skinWeights []components.SkinWeight
	if len(v1.SkinWeights) > 0 || len(v2.SkinWeights) > 0 {
		weightMap := map[*components.Bone]float64{}
		for _, sw := range v1.SkinWeights {
			weightMap[sw.Bone] = float64(sw.Weight) * (1 - t)
		}
		for _, sw := range v2.SkinWeights {
			weightMap[sw.Bone] += float64(sw.Weight) * t
		}
		for bone, weight := range weightMap {
			if weight > 0 {
				skinWeights = append(skinWeights, components.SkinWeight{Bone: bone, Weight: int(weight)})
			}
		}
	}

	var matrix *components.Matrix
	if v1.Matrix != nil {
		matrix = v1.Matrix
	} else if v2.Matrix != nil {
		matrix = v2.Matrix
	}

	out := &components.GeosetVertex{
		ID:          -1,
		Position:    interpVec3(v1.Position, v2.Position),
		Normal:      interpVec3(v1.Normal, v2.Normal),
		TexPosition: interpVec2(v1.TexPosition, v2.TexPosition),
		Matrix:      matrix,
	}
	if len(skinWeights) > 0 {
		out.SkinWeights = skinWeights
	}
	return out
}

func clipPolygon(verts []*components.GeosetVertex, plane clipPlane) []*components.GeosetVertex {
	inside := func(v *components.GeosetVertex) bool {
		if plane.min {
			return v.Position[plane.axis] >= plane.value
		}
		return v.Position[plane.axis] <= plane.value
	}
	var out []*components.GeosetVertex
	for i := 0; i < len(verts); i++ {
		curr := verts[i]
		next := verts[(i+1)%len(verts)]
		currIn := inside(curr)
		nextIn := inside(next)
		if currIn {
			out = append(out, curr)
		}
		if currIn != nextIn {
			denom := next.Position[plane.axis] - curr.Position[plane.axis]
			delta := 0.0
			if denom != 0 {
				delta = (plane.value - curr.Position[plane.axis]) / denom
			}
			out = append(out, interpolateVertex(curr, next, delta))
		}
	}
	return out
}
