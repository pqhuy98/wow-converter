package components

import (
	"fmt"
	stdmath "math"
	"strings"

	imath "github.com/pqhuy98/wow-converter/internal/math"
)

type Face struct {
	Vertices [3]*GeosetVertex
}

type Matrix struct {
	ID    int
	Bones []*Bone
}

type SkinWeight struct {
	Bone   *Bone
	Weight int
}

type GeosetWowData struct {
	SubmeshID int
}

type Geoset struct {
	Bound
	ID             int
	Name           string
	Vertices       []*GeosetVertex
	Faces          []Face
	Material       *Material
	Matrices       []Matrix
	SelectionGroup int
	Unselectable   bool
	WowData        GeosetWowData
}

type GeosetVertex struct {
	ID           int
	Position     imath.Vector3
	Normal       imath.Vector3
	TexPosition  imath.Vector2
	TexPosition2 *imath.Vector2
	Matrix       *Matrix
	SkinWeights  []SkinWeight
}

type GeosetAnim struct {
	ID         int
	Geoset     *Geoset
	DropShadow bool
	Alpha      *AnimatedOrStatic[float64]
	Color      *AnimatedOrStatic[imath.Vector3]
}

func GeosetsToString(version int, geosets []*Geoset, bones []*Bone, sequences []Sequence) string {
	getSkinWeight := func(vertex *GeosetVertex) string {
		boneIndices := make([]int, 4)
		weights := make([]int, 4)
		for i := 0; i < 4; i++ {
			if vertex.SkinWeights != nil && i < len(vertex.SkinWeights) {
				boneIndices[i] = vertex.SkinWeights[i].Bone.ObjectID
				weights[i] = vertex.SkinWeights[i].Weight
			}
		}
		parts := make([]string, 0, 8)
		for _, idx := range boneIndices {
			parts = append(parts, FVal(float64(idx)))
		}
		for _, w := range weights {
			parts = append(parts, FVal(float64(w)))
		}
		return "\t\t" + strings.Join(parts, ", ") + ","
	}

	var blocks []string
	for _, geoset := range geosets {
		useSkinWeights := false
		for _, v := range geoset.Vertices {
			if len(v.SkinWeights) > 0 {
				useSkinWeights = true
				break
			}
		}
		if version <= 800 && useSkinWeights {
			panic("Skin weights are not supported in MDL 800 or below")
		}

		useVertexGroup := false
		for _, v := range geoset.Vertices {
			if v.Matrix != nil {
				useVertexGroup = true
				break
			}
		}
		if useSkinWeights == useVertexGroup {
			panic("Geoset must not use skin weight and vertex group at the same time.")
		}
		if useSkinWeights {
			missing := 0
			for _, v := range geoset.Vertices {
				if len(v.SkinWeights) == 0 {
					missing++
				}
			}
			if missing > 0 {
				panic(fmt.Sprintf("Geoset %s has %d vertices without skin weights.", geoset.Name, missing))
			}
		}
		if useVertexGroup {
			missing := 0
			for _, v := range geoset.Vertices {
				if v.Matrix == nil {
					missing++
				}
			}
			if missing > 0 {
				panic(fmt.Sprintf("Geoset %s has %d vertices without vertex group.", geoset.Name, missing))
			}
		}

		for j, v := range geoset.Vertices {
			v.ID = j
		}
		for j := range geoset.Matrices {
			geoset.Matrices[j].ID = j
		}

		vertexGroupBlock := ""
		if useVertexGroup {
			var vg strings.Builder
			vg.WriteString("VertexGroup {\n")
			for _, v := range geoset.Vertices {
				vg.WriteString(FVal(float64(v.Matrix.ID)))
				vg.WriteString(",\n")
			}
			vg.WriteString("}\n")
			totalBones := 0
			for _, matrix := range geoset.Matrices {
				totalBones += len(matrix.Bones)
			}
			vg.WriteString("Groups ")
			vg.WriteString(FVal(float64(len(geoset.Matrices))))
			vg.WriteString(" ")
			vg.WriteString(FVal(float64(totalBones)))
			vg.WriteString(" {\n")
			for _, matrix := range geoset.Matrices {
				vg.WriteString("Matrices { ")
				parts := make([]string, len(matrix.Bones))
				for i, n := range matrix.Bones {
					parts[i] = FVal(float64(n.ObjectID))
				}
				vg.WriteString(strings.Join(parts, ", "))
				vg.WriteString(" },\n")
			}
			vg.WriteString("}\n")
			vertexGroupBlock = vg.String()
		}

		skinWeightsBlock := ""
		if useSkinWeights {
			normalToTangent := func(normal imath.Vector3) []float64 {
				sum := stdmath.Abs(normal[0] + normal[1] + normal[2])
				sign := 0.0
				if sum > 0 {
					sign = 1
				}
				return []float64{normal[0], normal[1], normal[2], sign}
			}
			var sw strings.Builder
			sw.WriteString("Tangents ")
			sw.WriteString(FVal(float64(len(geoset.Vertices))))
			sw.WriteString(" {\n")
			for _, v := range geoset.Vertices {
				sw.WriteString("{ ")
				sw.WriteString(FVector(normalToTangent(v.Normal)))
				sw.WriteString(" },\n")
			}
			sw.WriteString("}\n")
			sw.WriteString("SkinWeights ")
			sw.WriteString(FVal(float64(len(geoset.Vertices))))
			sw.WriteString(" {\n")
			for _, v := range geoset.Vertices {
				sw.WriteString(getSkinWeight(v))
				sw.WriteString("\n")
			}
			sw.WriteString("}\n")
			sw.WriteString("Groups ")
			sw.WriteString(FVal(float64(len(bones))))
			sw.WriteString(" ")
			sw.WriteString(FVal(float64(len(bones))))
			sw.WriteString(" {\n")
			for _, bone := range bones {
				sw.WriteString("Matrices { ")
				sw.WriteString(FVal(float64(bone.ObjectID)))
				sw.WriteString(" },\n")
			}
			sw.WriteString("}\n")
			skinWeightsBlock = sw.String()
		}

		hasCoord1 := false
		if geoset.Material != nil {
			for _, l := range geoset.Material.Layers {
				if l.CoordID != nil && *l.CoordID == 1 {
					hasCoord1 = true
					break
				}
			}
		}
		hasTex2 := false
		for _, v := range geoset.Vertices {
			if v.TexPosition2 != nil {
				hasTex2 = true
				break
			}
		}

		var b strings.Builder
		b.WriteString("Geoset {\n")
		b.WriteString("Vertices ")
		b.WriteString(FVal(float64(len(geoset.Vertices))))
		b.WriteString(" {\n")
		for _, vertex := range geoset.Vertices {
			b.WriteString("{ ")
			b.WriteString(FVector3(vertex.Position))
			b.WriteString(" },\n")
		}
		b.WriteString("}\n\n")
		b.WriteString("Normals ")
		b.WriteString(FVal(float64(len(geoset.Vertices))))
		b.WriteString(" {\n")
		for _, vertex := range geoset.Vertices {
			b.WriteString("{ ")
			b.WriteString(FVector3(vertex.Normal))
			b.WriteString(" },\n")
		}
		b.WriteString("}\n\n")
		b.WriteString("TVertices ")
		b.WriteString(FVal(float64(len(geoset.Vertices))))
		b.WriteString(" {\n")
		for _, vertex := range geoset.Vertices {
			b.WriteString("{ ")
			b.WriteString(FVector([]float64{vertex.TexPosition[0], vertex.TexPosition[1]}))
			b.WriteString(" },\n")
		}
		b.WriteString("}\n")
		if hasCoord1 && hasTex2 {
			b.WriteString("TVertices ")
			b.WriteString(FVal(float64(len(geoset.Vertices))))
			b.WriteString(" {\n")
			for _, vertex := range geoset.Vertices {
				tp := vertex.TexPosition
				if vertex.TexPosition2 != nil {
					tp = *vertex.TexPosition2
				}
				b.WriteString("{ ")
				b.WriteString(FVector([]float64{tp[0], tp[1]}))
				b.WriteString(" },\n")
			}
			b.WriteString("}\n")
		}
		b.WriteString(vertexGroupBlock)
		b.WriteString(skinWeightsBlock)
		b.WriteString("\nFaces 1 ")
		b.WriteString(FVal(float64(len(geoset.Faces) * 3)))
		b.WriteString(" {\n")
		b.WriteString("Triangles {\n")
		b.WriteString("{ ")
		var faceIDs []string
		for _, face := range geoset.Faces {
			for _, v := range face.Vertices {
				faceIDs = append(faceIDs, FVal(float64(v.ID)))
			}
		}
		b.WriteString(strings.Join(faceIDs, ", "))
		b.WriteString(" },\n")
		b.WriteString("}\n")
		b.WriteString("}\n\n")
		b.WriteString("MinimumExtent { ")
		b.WriteString(FVector3(geoset.MinimumExtent))
		b.WriteString(" },\n")
		b.WriteString("MaximumExtent { ")
		b.WriteString(FVector3(geoset.MaximumExtent))
		b.WriteString(" },\n")
		b.WriteString("BoundsRadius ")
		b.WriteString(FVal(geoset.BoundsRadius))
		b.WriteString(",\n\n")
		for _, seq := range sequences {
			b.WriteString("Anim {\n")
			b.WriteString("MinimumExtent { ")
			b.WriteString(FVector3(seq.MinimumExtent))
			b.WriteString(" },\n")
			b.WriteString("MaximumExtent { ")
			b.WriteString(FVector3(seq.MaximumExtent))
			b.WriteString(" },\n")
			b.WriteString("BoundsRadius ")
			b.WriteString(FVal(seq.BoundsRadius))
			b.WriteString(",\n")
			b.WriteString("}\n")
		}
		if geoset.Material != nil {
			b.WriteString("\nMaterialID ")
			b.WriteString(FVal(float64(geoset.Material.ID)))
			b.WriteString(",\n")
		}
		b.WriteString("SelectionGroup ")
		b.WriteString(FVal(float64(geoset.SelectionGroup)))
		b.WriteString(",\n")
		if geoset.Unselectable {
			b.WriteString("Unselectable,\n")
		}
		if version > 800 {
			b.WriteString("LevelOfDetail 0,\n")
			b.WriteString("Name \"")
			b.WriteString(geoset.Name)
			b.WriteString("\",\n")
		}
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}

func GeosetAnimsToString(geosetAnims []GeosetAnim) string {
	var blocks []string
	for _, geosetAnim := range geosetAnims {
		var b strings.Builder
		b.WriteString("GeosetAnim {\n")
		if geosetAnim.Geoset != nil {
			b.WriteString("GeosetId ")
			b.WriteString(FVal(float64(geosetAnim.Geoset.ID)))
			b.WriteString(",\n")
		}
		if geosetAnim.DropShadow {
			b.WriteString("DropShadow,\n")
		}
		if geosetAnim.Color != nil {
			b.WriteString(AnimatedValueToString("Color", geosetAnim.Color))
			b.WriteString("\n")
		}
		if geosetAnim.Alpha != nil {
			b.WriteString(AnimatedValueToString("Alpha", geosetAnim.Alpha))
			b.WriteString("\n")
		}
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}
