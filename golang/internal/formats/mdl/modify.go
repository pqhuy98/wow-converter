package mdl

import (
	"math"
	"regexp"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

type Modify struct {
	MDL *MDL
}

func NewModify(m *MDL) *Modify {
	return &Modify{MDL: m}
}

func (mod *Modify) OptimizeAll() *Modify {
	mod.SortSequences()
	mod.RemoveUnusedVertices()
	mod.RemoveUnusedNodes()
	mod.RemoveUnusedMaterialsTextures()
	mod.OptimizeKeyFrames()
	mod.MDL.Sync()
	return mod
}

func (mod *Modify) SetLargeBounds() *Modify {
	mod.MDL.BoundsOverriden = func(obj *components.Bound) {
		for i := 0; i < 3; i++ {
			absMin := obj.MinimumExtent[i]
			if absMin < 0 {
				absMin = -absMin
			}
			absMax := obj.MaximumExtent[i]
			if absMax < 0 {
				absMax = -absMax
			}
			abs := absMin
			if absMax > abs {
				abs = absMax
			}
			obj.MinimumExtent[i] = -abs * 3
			obj.MaximumExtent[i] = abs * 3
		}
		maxVal := obj.MaximumExtent[0]
		for i := 1; i < 3; i++ {
			if obj.MaximumExtent[i] > maxVal {
				maxVal = obj.MaximumExtent[i]
			}
		}
		obj.BoundsRadius = maxVal
	}
	return mod
}

func (mod *Modify) SetInfiniteBounds() *Modify {
	mod.MDL.BoundsOverriden = func(obj *components.Bound) {
		for i := 0; i < 3; i++ {
			obj.MinimumExtent[i] = -99999
			obj.MaximumExtent[i] = 99999
		}
		obj.BoundsRadius = 99999
	}
	return mod
}

func (mod *Modify) GetMaxZAtTimestamp(sequence components.Sequence, offset int) float64 {
	maxZ := math.Inf(-1)
	IterateVerticesAtTimestamp(mod.MDL, sequence, offset, func(_ *components.GeosetVertex, vPos imath.Vector3, _ *components.Geoset) {
		if vPos[2] > maxZ {
			maxZ = vPos[2]
		}
	})
	if math.IsInf(maxZ, -1) {
		return 0
	}
	return maxZ
}

func (mod *Modify) KeepCinematicSequences(patterns []any) *Modify {
	for i := range mod.MDL.Sequences {
		name := mod.MDL.Sequences[i].Data.WowName
		for _, p := range patterns {
			switch pat := p.(type) {
			case string:
				if strings.Contains(name, pat) {
					mod.MDL.Sequences[i].Keep = true
				}
			case *regexp.Regexp:
				if pat.MatchString(name) {
					mod.MDL.Sequences[i].Keep = true
				}
			}
		}
	}
	return mod
}

func (mod *Modify) ScaleParticlesDensity(factor float64) *Modify {
	for _, p := range mod.MDL.ParticleEmitter2s {
		if p.EmissionRate.Static {
			p.EmissionRate.Value *= factor
		} else if p.EmissionRate.Anim != nil {
			for k, v := range p.EmissionRate.Anim.KeyFrames {
				if fv, ok := v.(float64); ok {
					p.EmissionRate.Anim.KeyFrames[k] = fv * factor
				}
			}
		}
	}
	return mod
}

func (mod *Modify) CenterModelMinZ() *Modify {
	min := mod.MDL.Model.MinimumExtent
	max := mod.MDL.Model.MaximumExtent
	center := imath.V3Negative(imath.V3Mean(min, max))
	mod.Translate(imath.Vector3{center[0], center[1], min[2]})
	return mod
}
