package metadata

import (
	"math"
	"slices"

	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// The old JSON boundary converted non-finite numbers to null, then zero on
// typed decoding. Preserve that fallback without JSON, reflection or mutating
// the source loader. All nested slices are detached for assembly ownership.
func finite[T ~float32 | ~float64](v T) T {
	if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return 0
	}
	return v
}

func cloneValues(src [][]float64) [][]float64 {
	if src == nil {
		return nil
	}
	out := make([][]float64, len(src))
	for i, row := range src {
		out[i] = slices.Clone(row)
		for j := range out[i] {
			out[i][j] = finite(out[i][j])
		}
	}
	return out
}

func cloneTrack(src m2.Track) m2.Track {
	out := src
	out.Timestamps = slices.Clone(src.Timestamps)
	for i := range out.Timestamps {
		out.Timestamps[i] = slices.Clone(out.Timestamps[i])
	}
	out.Values = slices.Clone(src.Values)
	for i := range out.Values {
		out.Values[i] = cloneValues(out.Values[i])
	}
	return out
}

func clonePartTrack(src m2.PartTrack) m2.PartTrack {
	return m2.PartTrack{Timestamps: slices.Clone(src.Timestamps), Values: cloneValues(src.Values)}
}

func cloneCameras(src []m2.CameraEntry) []m2.CameraEntry {
	out := slices.Clone(src)
	for i := range out {
		v := &out[i]
		v.FarClip = finite(v.FarClip)
		v.NearClip = finite(v.NearClip)
		for j := range v.PositionBase {
			v.PositionBase[j] = finite(v.PositionBase[j])
		}
		for j := range v.TargetPositionBase {
			v.TargetPositionBase[j] = finite(v.TargetPositionBase[j])
		}
		v.FoV = cloneTrack(v.FoV)
	}
	return out
}

func cloneLights(src []m2.LightEntry) []m2.LightEntry {
	out := slices.Clone(src)
	for i := range out {
		v := &out[i]
		for j := range v.Position {
			v.Position[j] = finite(v.Position[j])
		}
		v.AmbientColor = cloneTrack(v.AmbientColor)
		v.AmbientIntensity = cloneTrack(v.AmbientIntensity)
		v.DiffuseColor = cloneTrack(v.DiffuseColor)
		v.DiffuseIntensity = cloneTrack(v.DiffuseIntensity)
		v.AttenuationStart = cloneTrack(v.AttenuationStart)
		v.AttenuationEnd = cloneTrack(v.AttenuationEnd)
		v.Visibility = cloneTrack(v.Visibility)
	}
	return out
}

func cloneRibbons(src []m2.RibbonEmitterEntry) []m2.RibbonEmitterEntry {
	out := slices.Clone(src)
	for i := range out {
		v := &out[i]
		for j := range v.Position {
			v.Position[j] = finite(v.Position[j])
		}
		v.TextureIndices = slices.Clone(v.TextureIndices)
		v.MaterialIndices = slices.Clone(v.MaterialIndices)
		v.ColorTrack = cloneTrack(v.ColorTrack)
		v.AlphaTrack = cloneTrack(v.AlphaTrack)
		v.HeightAboveTrack = cloneTrack(v.HeightAboveTrack)
		v.HeightBelowTrack = cloneTrack(v.HeightBelowTrack)
		v.EdgesPerSecond = finite(v.EdgesPerSecond)
		v.EdgeLifetime = finite(v.EdgeLifetime)
		v.Gravity = finite(v.Gravity)
		v.TexSlotTrack = cloneTrack(v.TexSlotTrack)
		v.VisibilityTrack = cloneTrack(v.VisibilityTrack)
	}
	return out
}

func cloneParticles(src []m2.ParticleEmitterEntry) []m2.ParticleEmitterEntry {
	out := slices.Clone(src)
	for i := range out {
		v := &out[i]
		for j := range v.Position {
			v.Position[j] = finite(v.Position[j])
		}
		for j := range v.MultiTextureScale {
			v.MultiTextureScale[j] = finite(v.MultiTextureScale[j])
		}
		for j := range v.MultiTextureScrollMid {
			for k := range v.MultiTextureScrollMid[j] {
				v.MultiTextureScrollMid[j][k] = finite(v.MultiTextureScrollMid[j][k])
			}
		}
		for j := range v.MultiTextureScrollRange {
			for k := range v.MultiTextureScrollRange[j] {
				v.MultiTextureScrollRange[j][k] = finite(v.MultiTextureScrollRange[j][k])
			}
		}
		v.EmissionSpeed = cloneTrack(v.EmissionSpeed)
		v.SpeedVariation = cloneTrack(v.SpeedVariation)
		v.VerticalRange = cloneTrack(v.VerticalRange)
		v.HorizontalRange = cloneTrack(v.HorizontalRange)
		v.Gravity = cloneTrack(v.Gravity)
		v.Lifespan = cloneTrack(v.Lifespan)
		v.LifespanVary = finite(v.LifespanVary)
		v.EmissionRate = cloneTrack(v.EmissionRate)
		v.EmissionAreaLength = cloneTrack(v.EmissionAreaLength)
		v.EmissionAreaWidth = cloneTrack(v.EmissionAreaWidth)
		v.ColorTrack = clonePartTrack(v.ColorTrack)
		v.AlphaTrack = clonePartTrack(v.AlphaTrack)
		v.ScaleTrack = clonePartTrack(v.ScaleTrack)
		for j := range v.ScaleVary {
			v.ScaleVary[j] = finite(v.ScaleVary[j])
		}
		v.HeadCellTrack = clonePartTrack(v.HeadCellTrack)
		v.TailCellTrack = clonePartTrack(v.TailCellTrack)
		v.TailLength = finite(v.TailLength)
		v.TwinkleScale.Min = finite(v.TwinkleScale.Min)
		v.TwinkleScale.Max = finite(v.TwinkleScale.Max)
		v.Drag = finite(v.Drag)
		v.EnabledIn = cloneTrack(v.EnabledIn)
	}
	return out
}

func cloneAnimationTrack(src Track) Track {
	out := src
	out.Timestamps = slices.Clone(src.Timestamps)
	for i := range out.Timestamps {
		out.Timestamps[i] = slices.Clone(out.Timestamps[i])
		for j, timestamp := range out.Timestamps[i] {
			if timestamp != nil {
				value := *timestamp
				out.Timestamps[i][j] = &value
			}
		}
	}
	out.Values = slices.Clone(src.Values)
	for i := range out.Values {
		out.Values[i] = cloneValues(out.Values[i])
	}
	return out
}
