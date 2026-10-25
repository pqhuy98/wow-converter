package creature

import (
	"math"
	"sort"

	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
)

// Approximate body size rather than enclosing tails, weapons and wings in a disk.
// Surface-area weighting keeps densely tessellated details from dominating height.
func creatureShadowDiameter(geosets []*mdx.Geoset) float64 {
	type heightSample struct{ z, weight float64 }
	var heights []heightSample
	projectedArea, totalWeight := 0.0, 0.0
	for _, g := range geosets {
		weights := make([]float64, len(g.Vertices)/3)
		for i := 0; i+2 < len(g.Faces); i += 3 {
			var vertices [3][3]float64
			valid := true
			for j := range vertices {
				index := int(g.Faces[i+j])
				if index >= len(weights) {
					valid = false
					break
				}
				for axis := range vertices[j] {
					v := float64(g.Vertices[3*index+axis])
					vertices[j][axis] = v
					valid = valid && !math.IsNaN(v) && !math.IsInf(v, 0)
				}
			}
			if !valid {
				continue
			}
			a, b := [3]float64{}, [3]float64{}
			for axis := range a {
				a[axis] = vertices[1][axis] - vertices[0][axis]
				b[axis] = vertices[2][axis] - vertices[0][axis]
			}
			cross := [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
			weight := math.Sqrt(cross[0]*cross[0]+cross[1]*cross[1]+cross[2]*cross[2]) / 6
			if weight <= 0 {
				continue
			}
			projectedArea += math.Abs(cross[2]) / 2
			for j := range vertices {
				weights[int(g.Faces[i+j])] += weight
			}
		}
		for i, weight := range weights {
			if weight > 0 {
				heights = append(heights, heightSample{float64(g.Vertices[3*i+2]), weight})
				totalWeight += weight
			}
		}
	}
	if len(heights) == 0 {
		return 2 * horizontalModelRadius(geosets)
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i].z < heights[j].z })
	low, high, accumulated := heights[0].z, heights[len(heights)-1].z, 0.0
	foundLow := false
	for _, sample := range heights {
		accumulated += sample.weight
		if !foundLow && accumulated >= .05*totalWeight {
			low, foundLow = sample.z, true
		}
		if accumulated >= .95*totalWeight {
			high = sample.z
			break
		}
	}
	// ponytail: half the two-sided projected area estimates a solid body's
	// footprint. Overlapping meshes can overestimate; rasterizing the posed
	// silhouette would be the next step if this approximation is insufficient.
	// Pad the equivalent disk by 25% for the blob's soft edge. The central 90%
	// body height prevents tall/narrow humanoids from receiving tiny shadows.
	return math.Max(high-low, 1.25*math.Sqrt(2*projectedArea/math.Pi))
}
