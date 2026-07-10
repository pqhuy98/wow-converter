package mdl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func (mod *Modify) ConvertToSd800() *Modify {
	mdl := mod.MDL
	if mdl.Version.FormatVersion == 800 {
		return mod
	}
	mdl.Version.FormatVersion = 800

	boneOriginalOrder := map[*components.Bone]int{}
	for idx, b := range mdl.Bones {
		boneOriginalOrder[b] = idx
	}
	nextBoneOrder := len(mdl.Bones)

	phantomBonesPerOriginal := map[*components.Bone][]*components.Bone{}
	createdPhantomBones := map[*components.Bone]struct{}{}
	nextPhantomIndexPerOriginal := map[*components.Bone]int{}

	ensurePhantomBones := func(original *components.Bone, requiredCount int) []*components.Bone {
		if _, ok := phantomBonesPerOriginal[original]; !ok {
			phantomBonesPerOriginal[original] = nil
			nextPhantomIndexPerOriginal[original] = 1
		}
		list := phantomBonesPerOriginal[original]
		for len(list) < requiredCount {
			idx := nextPhantomIndexPerOriginal[original]
			phantom := &components.Bone{
				NodeBase: components.NodeBase{
					Name:       fmt.Sprintf("%s__%d", original.Name, idx),
					Type:       "Bone",
					PivotPoint: original.PivotPoint,
					Parent:     original,
				},
				ParentBone: original,
			}
			mdl.Bones = append(mdl.Bones, phantom)
			createdPhantomBones[phantom] = struct{}{}
			boneOriginalOrder[phantom] = nextBoneOrder
			nextBoneOrder++
			list = append(list, phantom)
			nextPhantomIndexPerOriginal[original] = idx + 1
		}
		phantomBonesPerOriginal[original] = list
		return list
	}

	for _, geoset := range mdl.Geosets {
		matrixMap := map[string]*components.Matrix{}
		matrices := []components.Matrix{}

		getOrCreateMatrix := func(bones []*components.Bone) *components.Matrix {
			orders := make([]int, len(bones))
			for i, b := range bones {
				orders[i] = boneOriginalOrder[b]
			}
			sort.Ints(orders)
			keyParts := make([]string, len(orders))
			for i, o := range orders {
				keyParts[i] = fmt.Sprintf("%d", o)
			}
			key := strings.Join(keyParts, ",")
			if m, ok := matrixMap[key]; ok {
				return m
			}
			m := &components.Matrix{ID: len(matrices), Bones: bones}
			matrices = append(matrices, *m)
			matrixMap[key] = &matrices[len(matrices)-1]
			return &matrices[len(matrices)-1]
		}

		for _, v := range geoset.Vertices {
			const maxResolution = 8
			const tolerance = 0.02
			type weightEntry struct {
				bone   *components.Bone
				weight float64
			}
			var weights []weightEntry
			for _, sw := range v.SkinWeights {
				if sw.Weight > 0 {
					weights = append(weights, weightEntry{bone: sw.Bone, weight: float64(sw.Weight)})
				}
			}

			var bonesForMatrix []*components.Bone
			if len(weights) > 0 {
				sum := 0.0
				for _, w := range weights {
					sum += w.weight
				}
				if sum == 0 {
					sum = 1
				}
				normalized := make([]weightEntry, len(weights))
				for i, w := range weights {
					normalized[i] = weightEntry{bone: w.bone, weight: w.weight / sum}
				}

				type countEntry struct {
					bone  *components.Bone
					count int
				}
				var chosen []countEntry

				for k := 1; k <= maxResolution && len(chosen) == 0; k++ {
					type baseEntry struct {
						bone  *components.Bone
						count int
						frac  float64
					}
					base := make([]baseEntry, len(normalized))
					used := 0
					for i, w := range normalized {
						raw := w.weight * float64(k)
						count := int(raw)
						base[i] = baseEntry{bone: w.bone, count: count, frac: raw - float64(count)}
						used += count
					}
					if used < k {
						remaining := k - used
						sorted := append([]baseEntry(nil), base...)
						sort.Slice(sorted, func(i, j int) bool { return sorted[i].frac > sorted[j].frac })
						for i := 0; i < remaining; i++ {
							for bi := range base {
								if base[bi].bone == sorted[i].bone {
									base[bi].count++
									break
								}
							}
						}
					}
					maxError := 0.0
					for _, n := range normalized {
						for _, b := range base {
							if b.bone == n.bone {
								err := mathAbs(n.weight - float64(b.count)/float64(k))
								if err > maxError {
									maxError = err
								}
								break
							}
						}
					}
					if maxError <= tolerance {
						for _, b := range base {
							if b.count > 0 {
								chosen = append(chosen, countEntry{bone: b.bone, count: b.count})
							}
						}
					}
				}

				if len(chosen) == 0 {
					k := maxResolution
					type baseEntry struct {
						bone  *components.Bone
						count int
						frac  float64
					}
					base := make([]baseEntry, len(normalized))
					used := 0
					for i, w := range normalized {
						raw := w.weight * float64(k)
						count := int(raw)
						base[i] = baseEntry{bone: w.bone, count: count, frac: raw - float64(count)}
						used += count
					}
					if used < k {
						remaining := k - used
						sorted := append([]baseEntry(nil), base...)
						sort.Slice(sorted, func(i, j int) bool { return sorted[i].frac > sorted[j].frac })
						for i := 0; i < remaining; i++ {
							for bi := range base {
								if base[bi].bone == sorted[i].bone {
									base[bi].count++
									break
								}
							}
						}
					}
					for _, b := range base {
						if b.count > 0 {
							chosen = append(chosen, countEntry{bone: b.bone, count: b.count})
						}
					}
					if len(chosen) == 0 && len(normalized) > 0 {
						strongest := normalized[0]
						for _, n := range normalized[1:] {
							if n.weight > strongest.weight {
								strongest = n
							}
						}
						chosen = []countEntry{{bone: strongest.bone, count: 1}}
					}
				}

				for _, entry := range chosen {
					bonesForMatrix = append(bonesForMatrix, entry.bone)
					if entry.count > 1 {
						phantoms := ensurePhantomBones(entry.bone, entry.count-1)
						bonesForMatrix = append(bonesForMatrix, phantoms[:entry.count-1]...)
					}
				}
			} else if len(mdl.Bones) > 0 {
				bonesForMatrix = []*components.Bone{mdl.Bones[0]}
			}

			sort.Slice(bonesForMatrix, func(i, j int) bool {
				return boneOriginalOrder[bonesForMatrix[i]] < boneOriginalOrder[bonesForMatrix[j]]
			})

			mat := getOrCreateMatrix(bonesForMatrix)
			v.Matrix = mat
			v.SkinWeights = nil
		}

		if len(matrices) > 255 {
			type part struct {
				matrices  []components.Matrix
				matrixMap map[*components.Matrix]*components.Matrix
				vertices  []*components.GeosetVertex
				vertexMap map[*components.GeosetVertex]*components.GeosetVertex
				faces     []components.Face
			}
			var parts []part

			canFitFaceIntoPart := func(p *part, required map[*components.Matrix]struct{}) bool {
				needed := 0
				for m := range required {
					if _, ok := p.matrixMap[m]; !ok {
						needed++
					}
				}
				return len(p.matrices)+needed <= 255
			}

			ensureMatrixInPart := func(p *part, m *components.Matrix) *components.Matrix {
				if mapped, ok := p.matrixMap[m]; ok {
					return mapped
				}
				mapped := &components.Matrix{ID: len(p.matrices), Bones: m.Bones}
				p.matrices = append(p.matrices, *mapped)
				p.matrixMap[m] = mapped
				return mapped
			}

			for _, f := range geoset.Faces {
				required := map[*components.Matrix]struct{}{
					f.Vertices[0].Matrix: {},
					f.Vertices[1].Matrix: {},
					f.Vertices[2].Matrix: {},
				}
				var assigned *part
				for i := range parts {
					if canFitFaceIntoPart(&parts[i], required) {
						assigned = &parts[i]
						break
					}
				}
				if assigned == nil {
					parts = append(parts, part{
						matrixMap: map[*components.Matrix]*components.Matrix{},
						vertexMap: map[*components.GeosetVertex]*components.GeosetVertex{},
					})
					assigned = &parts[len(parts)-1]
				}

				vClones := make([]*components.GeosetVertex, 3)
				for i := 0; i < 3; i++ {
					origV := f.Vertices[i]
					clone, ok := assigned.vertexMap[origV]
					if !ok {
						mappedMatrix := ensureMatrixInPart(assigned, origV.Matrix)
						clone = &components.GeosetVertex{
							ID:           origV.ID,
							Position:     origV.Position,
							Normal:       origV.Normal,
							TexPosition:  origV.TexPosition,
							TexPosition2: origV.TexPosition2,
							Matrix:       mappedMatrix,
						}
						assigned.vertices = append(assigned.vertices, clone)
						assigned.vertexMap[origV] = clone
					} else {
						clone.Matrix = ensureMatrixInPart(assigned, origV.Matrix)
					}
					vClones[i] = clone
				}
				assigned.faces = append(assigned.faces, components.Face{Vertices: [3]*components.GeosetVertex{vClones[0], vClones[1], vClones[2]}})
			}

			var newGeosets []*components.Geoset
			for idx, p := range parts {
				newGeosets = append(newGeosets, &components.Geoset{
					Bound:          geoset.Bound,
					ID:             -1,
					Name:           fmt.Sprintf("%s_%d", geoset.Name, idx),
					Vertices:       p.vertices,
					Faces:          p.faces,
					Material:       geoset.Material,
					Matrices:       p.matrices,
					SelectionGroup: geoset.SelectionGroup,
					Unselectable:   geoset.Unselectable,
					WowData:        geoset.WowData,
				})
			}
			if len(newGeosets) > 0 {
				first := newGeosets[0]
				*geoset = *first
				mdl.Geosets = append(mdl.Geosets, newGeosets[1:]...)
			}
		} else {
			geoset.Matrices = matrices
		}
	}

	usedBones := map[*components.Bone]struct{}{}
	for _, gs := range mdl.Geosets {
		for _, m := range gs.Matrices {
			for _, b := range m.Bones {
				usedBones[b] = struct{}{}
			}
		}
	}
	var filteredBones []*components.Bone
	for _, b := range mdl.Bones {
		if _, created := createdPhantomBones[b]; !created {
			filteredBones = append(filteredBones, b)
			continue
		}
		if _, used := usedBones[b]; used {
			filteredBones = append(filteredBones, b)
		}
	}
	mdl.Bones = filteredBones

	for _, gs := range mdl.Geosets {
		if len(gs.Matrices) > 255 {
			panic("Geoset matrices exceed length 255")
		}
		seen := map[*components.Matrix]struct{}{}
		for i := range gs.Matrices {
			ptr := &gs.Matrices[i]
			if _, ok := seen[ptr]; ok {
				panic("Geoset matrices are not unique")
			}
			seen[ptr] = struct{}{}
		}
	}

	mod.OptimizeKeyFrames()
	mod.sortKeyframes()
	mdl.Sync()
	return mod
}

func (mod *Modify) sortKeyframes() {
	animated := mod.MDL.GetAnimated()
	newIntervals := map[int][2]int{}
	accum := 1
	for i, s := range mod.MDL.Sequences {
		newInterval := [2]int{accum, accum + s.Interval[1] - s.Interval[0]}
		newIntervals[i] = newInterval
		accum = newInterval[1] + 1
	}

	sortedSeqIdx := make([]int, len(mod.MDL.Sequences))
	for i := range sortedSeqIdx {
		sortedSeqIdx[i] = i
	}
	sort.Slice(sortedSeqIdx, func(i, j int) bool {
		return mod.MDL.Sequences[sortedSeqIdx[i]].Interval[0] < mod.MDL.Sequences[sortedSeqIdx[j]].Interval[0]
	})

	for _, a := range animated {
		if a.GlobalSeq != nil {
			continue
		}
		curSeqIdx := 0
		timestamps := components.SortedKeyInts(a.KeyFrames)
		var entries [][2]any
		for _, t := range timestamps {
			for curSeqIdx < len(sortedSeqIdx) && mod.MDL.Sequences[sortedSeqIdx[curSeqIdx]].Interval[1] < t {
				curSeqIdx++
			}
			if curSeqIdx >= len(sortedSeqIdx) {
				continue
			}
			seq := mod.MDL.Sequences[sortedSeqIdx[curSeqIdx]]
			if t < seq.Interval[0] || seq.Interval[1] < t {
				continue
			}
			seqOrigIdx := sortedSeqIdx[curSeqIdx]
			newTimestamp := newIntervals[seqOrigIdx][0] + t - seq.Interval[0]
			entries = append(entries, [2]any{newTimestamp, a.KeyFrames[t]})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i][0].(int) < entries[j][0].(int)
		})
		a.KeyFrames = map[int]any{}
		for _, e := range entries {
			a.KeyFrames[e[0].(int)] = e[1]
		}
	}

	for i := range mod.MDL.Sequences {
		mod.MDL.Sequences[i].Interval = newIntervals[i]
	}
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

