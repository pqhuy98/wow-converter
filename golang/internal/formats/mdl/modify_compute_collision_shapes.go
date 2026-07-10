package mdl

import (
	"fmt"
	"math"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func (mod *Modify) AddCollisionShapes() *Modify {
	var seq *components.Sequence
	for i := range mod.MDL.Sequences {
		if mod.MDL.Sequences[i].Name == "Stand" {
			seq = &mod.MDL.Sequences[i]
			break
		}
	}
	if seq == nil && len(mod.MDL.Sequences) > 0 {
		seq = &mod.MDL.Sequences[0]
	}
	if seq == nil {
		return mod
	}

	vToPos := map[*components.GeosetVertex]imath.Vector3{}
	timestamp := seq.Interval[0]
	IterateVerticesAtTimestamp(mod.MDL, *seq, timestamp, func(v *components.GeosetVertex, vPos imath.Vector3, _ *components.Geoset) {
		if !isFinite(vPos[0]) || !isFinite(vPos[1]) || !isFinite(vPos[2]) {
			return
		}
		vToPos[v] = vPos
	})

	var cloud []imath.Vector3
	for _, geoset := range mod.MDL.Geosets {
		for _, face := range geoset.Faces {
			v0, ok0 := vToPos[face.Vertices[0]]
			v1, ok1 := vToPos[face.Vertices[1]]
			v2, ok2 := vToPos[face.Vertices[2]]
			if !ok0 || !ok1 || !ok2 {
				continue
			}
			centroid := imath.Vector3{
				(v0[0] + v1[0] + v2[0]) / 3,
				(v0[1] + v1[1] + v2[1]) / 3,
				(v0[2] + v1[2] + v2[2]) / 3,
			}
			if isFinite(centroid[0]) && isFinite(centroid[1]) && isFinite(centroid[2]) {
				cloud = append(cloud, centroid)
			}
		}
	}

	if len(cloud) == 0 {
		for _, geoset := range mod.MDL.Geosets {
			for _, v := range geoset.Vertices {
				p := v.Position
				if isFinite(p[0]) && isFinite(p[1]) && isFinite(p[2]) {
					cloud = append(cloud, p)
				}
			}
		}
	}

	if len(cloud) == 0 {
		const bufferRadius = 25.0
		min := imath.Vector3{
			mod.MDL.Model.MinimumExtent[0] - bufferRadius,
			mod.MDL.Model.MinimumExtent[1] - bufferRadius,
			mod.MDL.Model.MinimumExtent[2] - bufferRadius,
		}
		max := imath.Vector3{
			mod.MDL.Model.MaximumExtent[0] + bufferRadius,
			mod.MDL.Model.MaximumExtent[1] + bufferRadius,
			mod.MDL.Model.MaximumExtent[2] + bufferRadius,
		}
		mod.MDL.CollisionShapes = []*components.CollisionShape{{
			NodeBase:    components.NodeBase{Name: "Collision Box01", Type: "Box", PivotPoint: imath.Vector3{}},
			ShapeType:   "Box",
			Vertices:    []imath.Vector3{min, max},
			BoundRadius: 0,
		}}
		return mod
	}

	const bufferRadius = 5.0
	type cluster struct {
		pointsIdx []int
		rawMin    imath.Vector3
		rawMax    imath.Vector3
		min       imath.Vector3
		max       imath.Vector3
		volume    float64
	}

	computeAABB := func(idxs []int) cluster {
		rawMin := imath.Vector3{math.Inf(1), math.Inf(1), math.Inf(1)}
		rawMax := imath.Vector3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
		for _, idx := range idxs {
			p := cloud[idx]
			for axis := 0; axis < 3; axis++ {
				if p[axis] < rawMin[axis] {
					rawMin[axis] = p[axis]
				}
				if p[axis] > rawMax[axis] {
					rawMax[axis] = p[axis]
				}
			}
		}
		min := imath.Vector3{
			rawMin[0] - bufferRadius, rawMin[1] - bufferRadius, rawMin[2] - bufferRadius,
		}
		max := imath.Vector3{
			rawMax[0] + bufferRadius, rawMax[1] + bufferRadius, rawMax[2] + bufferRadius,
		}
		dx := max[0] - min[0]
		dy := max[1] - min[1]
		dz := max[2] - min[2]
		if dx < 0 {
			dx = 0
		}
		if dy < 0 {
			dy = 0
		}
		if dz < 0 {
			dz = 0
		}
		return cluster{
			pointsIdx: idxs, rawMin: rawMin, rawMax: rawMax, min: min, max: max,
			volume: dx * dy * dz,
		}
	}

	initialIdxs := make([]int, len(cloud))
	for i := range cloud {
		initialIdxs[i] = i
	}
	clusters := []cluster{computeAABB(initialIdxs)}

	const maxShapes = 3
	const numBins = 16

	for len(clusters) < maxShapes {
		bestGain := 0.0
		var bestSplit *struct {
			clusterIndex int
			left, right  cluster
		}

		for ci := range clusters {
			c := clusters[ci]
			tryAxis := func(axis int) {
				minVal := c.rawMin[axis]
				maxVal := c.rawMax[axis]
				if !(maxVal > minVal) {
					return
				}
				for b := 1; b < numBins; b++ {
					thr := minVal + (float64(b) * (maxVal - minVal) / numBins)
					var leftIdx, rightIdx []int
					for _, idx := range c.pointsIdx {
						if cloud[idx][axis] <= thr {
							leftIdx = append(leftIdx, idx)
						} else {
							rightIdx = append(rightIdx, idx)
						}
					}
					if len(leftIdx) == 0 || len(rightIdx) == 0 {
						continue
					}
					l := computeAABB(leftIdx)
					r := computeAABB(rightIdx)
					gain := c.volume - (l.volume + r.volume)
					if gain > bestGain {
						bestGain = gain
						bestSplit = &struct {
							clusterIndex int
							left, right  cluster
						}{clusterIndex: ci, left: l, right: r}
					}
				}
			}
			tryAxis(0)
			tryAxis(1)
			tryAxis(2)
		}

		if bestSplit == nil || bestGain <= 0 {
			break
		}
		i := bestSplit.clusterIndex
		clusters = append(clusters[:i], append([]cluster{bestSplit.left, bestSplit.right}, clusters[i+1:]...)...)
	}

	mod.MDL.CollisionShapes = nil
	for i, c := range clusters {
		mod.MDL.CollisionShapes = append(mod.MDL.CollisionShapes, &components.CollisionShape{
			NodeBase:    components.NodeBase{Name: fmt.Sprintf("Collision Box%02d", i+1), Type: "Box", PivotPoint: imath.Vector3{}},
			ShapeType:   "Box",
			Vertices:    []imath.Vector3{c.min, c.max},
			BoundRadius: 0,
		})
	}
	return mod
}

func isFinite(v float64) bool {
	return !math.IsInf(v, 0) && !math.IsNaN(v)
}
