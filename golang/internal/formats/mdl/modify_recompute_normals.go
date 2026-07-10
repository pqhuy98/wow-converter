package mdl

import (
	"fmt"
	"log"
	"math"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func (mod *Modify) RecomputeNormals() *Modify {
	const posEps = 1e-2
	q := func(x float64) float64 {
		v := math.Round(x/posEps) * posEps
		if v == 0 {
			return 0
		}
		return v
	}
	keyFromPosition := func(pos imath.Vector3) string {
		return fmt.Sprintf("%v|%v|%v", q(pos[0]), q(pos[1]), q(pos[2]))
	}

	vertexToFaces := map[*components.GeosetVertex][]components.Face{}
	positionToVertices := map[string][]*components.GeosetVertex{}

	for _, geoset := range mod.MDL.Geosets {
		for _, vert := range geoset.Vertices {
			vert.Normal = imath.Vector3{}
			key := keyFromPosition(vert.Position)
			positionToVertices[key] = append(positionToVertices[key], vert)
		}
		for _, face := range geoset.Faces {
			for _, vert := range face.Vertices {
				vertexToFaces[vert] = append(vertexToFaces[vert], face)
			}
		}
	}

	sub := func(a, b imath.Vector3) imath.Vector3 {
		return imath.Vector3{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
	}
	cross := func(a, b imath.Vector3) imath.Vector3 {
		return imath.Vector3{
			a[1]*b[2] - a[2]*b[1],
			a[2]*b[0] - a[0]*b[2],
			a[0]*b[1] - a[1]*b[0],
		}
	}
	length := func(v imath.Vector3) float64 {
		return math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	}

	for _, verticesAtPos := range positionToVertices {
		sum := imath.Vector3{}
		for _, vert := range verticesAtPos {
			for _, face := range vertexToFaces[vert] {
				v0, v1, v2 := face.Vertices[0], face.Vertices[1], face.Vertices[2]
				e1 := sub(v0.Position, v1.Position)
				e2 := sub(v1.Position, v2.Position)
				perp := cross(e1, e2)
				sum[0] += perp[0]
				sum[1] += perp[1]
				sum[2] += perp[2]
			}
		}
		sumMag := length(sum)
		if sumMag == 0 {
			sum = imath.Vector3{0, 0, 1}
			sumMag = 1
		}
		normal := imath.Vector3{sum[0] / sumMag, sum[1] / sumMag, sum[2] / sumMag}
		for _, vert := range verticesAtPos {
			vert.Normal = normal
		}
	}
	return mod
}

func (mod *Modify) RemoveSmallFaceComponents(minComponentArea float64) *Modify {
	deletedFaces := map[components.Face]struct{}{}

	for _, geoset := range mod.MDL.Geosets {
		if len(geoset.Faces) == 0 {
			continue
		}
		faces := geoset.Faces
		faceCount := len(faces)

		vertexToFaces := map[*components.GeosetVertex][]int{}
		for i, face := range faces {
			for _, vertex := range face.Vertices {
				vertexToFaces[vertex] = append(vertexToFaces[vertex], i)
			}
		}

		neighbours := make([][]int, faceCount)
		for _, indices := range vertexToFaces {
			for i := 0; i < len(indices); i++ {
				for j := i + 1; j < len(indices); j++ {
					a, b := indices[i], indices[j]
					neighbours[a] = append(neighbours[a], b)
					neighbours[b] = append(neighbours[b], a)
				}
			}
		}

		visited := make([]bool, faceCount)
		for i := 0; i < faceCount; i++ {
			if visited[i] {
				continue
			}
			stack := []int{i}
			visited[i] = true
			var componentFaces []int
			componentArea := 0.0

			for len(stack) > 0 {
				idx := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				componentFaces = append(componentFaces, idx)
				componentArea += faceArea(faces[idx])

				for _, next := range neighbours[idx] {
					if !visited[next] {
						visited[next] = true
						stack = append(stack, next)
					}
				}
			}

			if componentArea < minComponentArea {
				for _, idx := range componentFaces {
					deletedFaces[faces[idx]] = struct{}{}
				}
			}
		}
	}

	log.Printf("deletedFaces: %d", len(deletedFaces))
	mod.DeleteFacesIf(func(f components.Face, _ *components.Geoset) bool {
		_, ok := deletedFaces[f]
		return ok
	})
	return mod
}

func faceArea(face components.Face) float64 {
	a, b, c := face.Vertices[0].Position, face.Vertices[1].Position, face.Vertices[2].Position
	abX, abY, abZ := b[0]-a[0], b[1]-a[1], b[2]-a[2]
	acX, acY, acZ := c[0]-a[0], c[1]-a[1], c[2]-a[2]
	crossX := abY*acZ - abZ*acY
	crossY := abZ*acX - abX*acZ
	crossZ := abX*acY - abY*acX
	crossLength := math.Sqrt(crossX*crossX + crossY*crossY + crossZ*crossZ)
	return 0.5 * crossLength
}
