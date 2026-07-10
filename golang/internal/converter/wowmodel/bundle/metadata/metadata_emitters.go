package metadata

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	bundleutils "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/utils"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// ExtractMDLCameras builds WC3 camera definitions from M2 camera data.
func (f *File) ExtractMDLCameras() {
	if !f.IsLoaded || f.mdl == nil || len(f.cameras) == 0 {
		return
	}
	const aspect = 4.0 / 3.0
	cams := append([]m2.CameraEntry(nil), f.cameras...)
	sort.Slice(cams, func(i, j int) bool {
		return cameraScore(cams[i].Type) < cameraScore(cams[j].Type)
	})
	for i, c := range cams {
		pos := imath.Vector3{float64(c.PositionBase[0]), float64(-c.PositionBase[2]), float64(c.PositionBase[1])}
		target := imath.Vector3{float64(c.TargetPositionBase[0]), float64(-c.TargetPositionBase[2]), float64(c.TargetPositionBase[1])}
		vfov := 1.0
		if len(c.FoV.Values) > 0 && len(c.FoV.Values[0]) > 0 && len(c.FoV.Values[0][0]) > 0 {
			dfov := c.FoV.Values[0][0][0]
			vfov = dfov / math.Sqrt(1+aspect*aspect)
		}
		pos = scaleFromTarget(pos, target, 2)
		f.mdl.Cameras = append(f.mdl.Cameras, components.Camera{
			Name:        cameraTypeName(c.Type, i),
			FieldOfView: vfov,
			NearClip:    0.1,
			FarClip:     float64(c.FarClip) * 4,
			Position:    pos,
			Target:      components.CameraTarget{Position: target},
		})
	}
	if !f.Config.IsBulkExport && len(f.mdl.Cameras) > 0 {
		log.Printf("Cameras: %d", len(f.mdl.Cameras))
	}
}

func cameraScore(t int32) int {
	switch t {
	case 0:
		return 0
	case 1:
		return 1
	case -1:
		return 2
	default:
		return 3
	}
}

func cameraTypeName(t int32, i int) string {
	switch t {
	case 0:
		return "Portrait_Camera"
	case 1:
		return "CharacterInfo_Camera"
	case -1:
		return "Flyby_Camera_" + itoa(i)
	default:
		return "Camera_" + itoa(i)
	}
}

func scaleFromTarget(pos, target imath.Vector3, scale float64) imath.Vector3 {
	return imath.Vector3{
		target[0] + (pos[0]-target[0])*scale,
		target[1] + (pos[1]-target[1])*scale,
		target[2] + (pos[2]-target[2])*scale,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// ExtractMDLLights builds WC3 lights from M2 light data.
func (f *File) ExtractMDLLights() {
	if !f.IsLoaded || f.mdl == nil || len(f.lights) == 0 || len(f.mdl.Bones) == 0 {
		return
	}
	scale15 := func(v []float64) any { return v[0] * 1.5 }
	for i, l := range f.lights {
		parent := f.mdl.Bones[0]
		if l.Bone >= 0 && int(l.Bone) < len(f.mdl.Bones) {
			parent = f.mdl.Bones[l.Bone]
		}
		lightType := components.LightDirectional
		if l.Type == 1 {
			lightType = components.LightOmnidirectional
		}
		vis := f.m2trackToAnimation(l.Visibility, components.AnimTypeOthers, scalarIdentity)
		f.mdl.Lights = append(f.mdl.Lights, &components.Light{
			NodeBase: components.NodeBase{
				Name:       "Light_" + itoa(i),
				Type:       "Light",
				Parent:     parent,
				PivotPoint: imath.Vector3{float64(l.Position[0]), float64(-l.Position[2]), float64(l.Position[1])},
			},
			LightType:        lightType,
			AttenuationStart: f.m2trackToAnimationOrStaticFloat(l.AttenuationStart, components.AnimTypeOthers, scale15),
			AttenuationEnd:   f.m2trackToAnimationOrStaticFloat(l.AttenuationEnd, components.AnimTypeOthers, scale15),
			Intensity:        f.m2trackToAnimationOrStaticFloat(l.DiffuseIntensity, components.AnimTypeOthers, scale15),
			Color:            f.m2trackToAnimationOrStaticVec3(l.DiffuseColor, components.AnimTypeColor, vec3Identity),
			AmbientIntensity: f.m2trackToAnimationOrStaticFloat(l.AmbientIntensity, components.AnimTypeOthers, scale15),
			AmbientColor:     f.m2trackToAnimationOrStaticVec3(l.AmbientColor, components.AnimTypeColor, vec3Identity),
			Visibility:       vis,
		})
	}
	if !f.Config.IsBulkExport && len(f.mdl.Lights) > 0 {
		log.Printf("%s %s %d", ansi.Yellow("Lights:"), f.mdl.Model.Name, len(f.mdl.Lights))
	}
}

func vec3Identity(v []float64) any {
	if len(v) < 3 {
		return imath.Vector3{}
	}
	return imath.Vector3{v[0], v[1], v[2]}
}

// ExtractMDLRibbonEmitters builds WC3 ribbon emitters from M2 ribbon data.
func (f *File) ExtractMDLRibbonEmitters(textures []components.Texture) {
	if !f.IsLoaded || f.mdl == nil || len(f.ribbonEmitters) == 0 || len(f.mdl.Bones) == 0 {
		return
	}
	for i, r := range f.ribbonEmitters {
		texIdx := 0
		if len(r.TextureIndices) > 0 {
			texIdx = int(r.TextureIndices[0])
		}
		if texIdx < 0 || texIdx >= len(textures) {
			payload, _ := json.Marshal(map[string]any{
				"model":          f.mdl.Model.Name,
				"ribbonIndex":      i,
				"textureIndex":     texIdx,
				"texturesLength":   len(textures),
			})
			log.Printf("%s %s", ansi.Red("Ribbon with invalid texture index"), string(payload))
			continue
		}
		matIdx := 0
		if len(r.MaterialIndices) > 0 {
			matIdx = int(r.MaterialIndices[0])
		}
		if matIdx >= len(f.materials) {
			payload, _ := json.Marshal(map[string]any{
				"model":           f.mdl.Model.Name,
				"ribbonIndex":     i,
				"materialIndex":   matIdx,
				"materialsLength": len(f.materials),
			})
			log.Printf("%s %s", ansi.Red("Ribbon with invalid material index"), string(payload))
			continue
		}
		material := f.materials[matIdx]
		textAnimID := -1
		if int(r.TextureTransformLookupIndex) >= 0 && int(r.TextureTransformLookupIndex) < len(f.textureTransformsLookup) {
			textAnimID = f.textureTransformsLookup[r.TextureTransformLookupIndex]
		}
		filterMode := bundleutils.GetLayerFilterMode(uint16(material.BlendingMode), 0, 0, textures[texIdx].Image)
		if filterMode == nil {
			continue
		}
		ribbonMat := &components.Material{
			TwoSided: material.Flags&0x04 > 0,
			Layers: []components.Layer{{
				FilterMode:  *filterMode,
				Texture:     storeMdlTexture(f.mdl, &textures[texIdx]),
				Alpha:       components.AnimatedOrStatic[float64]{Static: true, Value: 1},
				Unlit:       material.Flags&0x01 > 0,
				Unfogged:    material.Flags&0x02 > 0,
				TwoSided:    material.Flags&0x04 > 0,
				NoDepthTest: material.Flags&0x08 > 0,
				NoDepthSet:  material.Flags&0x10 > 0,
			}},
		}
		if textAnimID >= 0 && textAnimID < len(f.mdl.TextureAnims) {
			ribbonMat.Layers[0].TVertexAnim = &f.mdl.TextureAnims[textAnimID]
		}
		f.mdl.Materials = append(f.mdl.Materials, ribbonMat)
		parent := f.mdl.Bones[0]
		if r.BoneIndex < uint32(len(f.mdl.Bones)) {
			parent = f.mdl.Bones[r.BoneIndex]
		}
		heightAbove := ptrAnimOrStatic(f.m2trackToAnimationOrStaticFloat(r.HeightAboveTrack, components.AnimTypeOthers, scalarIdentity))
		heightBelow := ptrAnimOrStatic(f.m2trackToAnimationOrStaticFloat(r.HeightBelowTrack, components.AnimTypeOthers, scalarIdentity))
		alpha := ptrAnimOrStatic(f.m2trackToAnimationOrStaticFloat(r.AlphaTrack, components.AnimTypeAlpha, func(v []float64) any { return v[0] / 32767 }))
		color := ptrAnimOrStaticVec3(f.m2trackToAnimationOrStaticVec3(r.ColorTrack, components.AnimTypeColor, vec3Identity))
		texSlot := ptrAnimOrStatic(f.m2trackToAnimationOrStaticFloat(r.TexSlotTrack, components.AnimTypeOthers, scalarIdentity))
		f.mdl.RibbonEmitters = append(f.mdl.RibbonEmitters, &components.RibbonEmitter{
			NodeBase: components.NodeBase{
				Name:       "RibbonEmitter_" + itoa(i),
				Type:       "RibbonEmitter",
				Parent:     parent,
				PivotPoint: imath.Vector3{float64(r.Position[0]), float64(-r.Position[2]), float64(r.Position[1])},
			},
			HeightAbove:  heightAbove,
			HeightBelow:  heightBelow,
			Alpha:        alpha,
			Color:        color,
			TextureSlot:  texSlot,
			Visibility:   f.m2trackToAnimation(r.VisibilityTrack, components.AnimTypeOthers, scalarIdentity),
			EmissionRate: float64(r.EdgesPerSecond),
			LifeSpan:     float64(r.EdgeLifetime),
			Rows:         maxInt(1, int(r.TextureRows)),
			Columns:      maxInt(1, int(r.TextureCols)),
			MaterialID:   0,
			Gravity:      float64(r.Gravity),
		})
	}
	if !f.Config.IsBulkExport && len(f.mdl.RibbonEmitters) > 0 {
		log.Printf("%s %d", ansi.Yellow("Ribbon emitters:"), len(f.mdl.RibbonEmitters))
	}
}

func scalarIdentity(v []float64) any {
	if len(v) == 0 {
		return float64(0)
	}
	return v[0]
}

func ptrAnimOrStatic(v components.AnimatedOrStatic[float64]) *components.AnimatedOrStatic[float64] {
	return &v
}

func ptrAnimOrStaticVec3(v components.AnimatedOrStatic[imath.Vector3]) *components.AnimatedOrStatic[imath.Vector3] {
	return &v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ExtractMDLParticlesEmitters builds WC3 particle emitters from M2 particle data.
func (f *File) ExtractMDLParticlesEmitters(textures []components.Texture) {
	if !f.IsLoaded || f.mdl == nil || len(f.particleEmitters) == 0 || len(f.mdl.Bones) == 0 {
		return
	}
	for i, p := range f.particleEmitters {
		textureID := int(p.TexturePacked)
		if p.Flags&0x10000000 != 0 {
			textureID &= 0x1F
		}
		if textureID < 0 || textureID >= len(textures) {
			continue
		}
		parent := f.mdl.Bones[0]
		if int(p.Bone) < len(f.mdl.Bones) {
			parent = f.mdl.Bones[p.Bone]
		}
		hasHead := p.Flags&0x20000 != 0
		hasTail := p.Flags&0x40000 != 0
		headOrTail := components.HeadOrTailHead
		if hasHead && hasTail {
			headOrTail = components.HeadOrTailBoth
		} else if hasTail {
			headOrTail = components.HeadOrTailTail
		}
		baseEmissionRate := f.m2trackToAnimationOrStaticFloat(p.EmissionRate, components.AnimTypeOthers, scalarIdentity)
		lifeSpan, tailLength, emissionRate := correctParticleEmission(p, baseEmissionRate)
		speed := f.m2trackToAnimationOrStaticFloat(p.EmissionSpeed, components.AnimTypeOthers, scalarIdentity)
		node := &components.ParticleEmitter2{
			NodeBase: components.NodeBase{
				Name:       "ParticleEmitter_" + itoa(i),
				Type:       "ParticleEmitter2",
				Parent:     parent,
				PivotPoint: imath.Vector3{float64(p.Position[0]), float64(-p.Position[2]), float64(p.Position[1])},
			},
			FilterMode:         mapParticleBlend(p.BlendingType),
			Width:              f.m2trackToAnimationOrStaticFloat(p.EmissionAreaWidth, components.AnimTypeOthers, scalarIdentity),
			Length:             f.m2trackToAnimationOrStaticFloat(p.EmissionAreaLength, components.AnimTypeOthers, scalarIdentity),
			Speed:              speed,
			Variation:          f.m2trackToAnimationOrStaticFloat(p.SpeedVariation, components.AnimTypeOthers, scalarIdentity),
			EmissionRate:       emissionRate,
			Latitude:           f.m2trackToAnimationOrStaticFloat(p.VerticalRange, components.AnimTypeOthers, degrees),
			Visibility:         f.m2trackToAnimation(p.EnabledIn, components.AnimTypeOthers, scalarIdentity),
			Texture:            storeMdlTexture(f.mdl, &textures[textureID]),
			TailLength:         tailLength,
			Columns:            maxInt(1, int(p.TextureCols)),
			Rows:               maxInt(1, int(p.TextureRows)),
			HeadOrTail:         headOrTail,
			Gravity:            f.m2trackToAnimationOrStaticFloat(p.Gravity, components.AnimTypeOthers, decompressGravity),
			LifeSpan:           lifeSpan,
			TimeMiddle:         particleTimeMiddle(p),
			Squirt:             p.Flags&0x40 != 0,
			SegmentColors:      particleSegmentColors(p),
			SegmentAlphas:      particleSegmentAlphas(p),
			SegmentScaling:     particleSegmentScaling(p),
			HeadIntervals:      particleHeadIntervals(p),
			DecayIntervals:     particleDecayIntervals(p),
			TailIntervals:      particleTailIntervals(p),
			TailDecayIntervals: particleTailDecayIntervals(p),
		}
		if p.Drag > 0 {
			applyParticleDrag(&node.Speed, node.LifeSpan, float64(p.Drag))
		}
		clampParticleUVIntervals(node)
		if p.Flags&0x1 != 0 {
			node.Flags2 = append(node.Flags2, components.PE2Unshaded)
		}
		if p.Flags&0x10 != 0 {
			node.Flags2 = append(node.Flags2, components.PE2ModelSpace)
		}
		f.mdl.ParticleEmitter2s = append(f.mdl.ParticleEmitter2s, node)
	}
	if !f.Config.IsBulkExport && len(f.particleEmitters) > 0 {
		log.Printf("Particle emitters: %d", len(f.particleEmitters))
	}
}

func clampParticleUVIntervals(node *components.ParticleEmitter2) {
	clamp := func(uv [3]float64) [3]float64 {
		uv[0] = clampFloat(uv[0], 0, float64(node.Rows-1))
		uv[1] = clampFloat(uv[1], 0, float64(node.Columns-1))
		return uv
	}
	node.HeadIntervals = clamp(node.HeadIntervals)
	node.DecayIntervals = clamp(node.DecayIntervals)
	node.TailIntervals = clamp(node.TailIntervals)
	node.TailDecayIntervals = clamp(node.TailDecayIntervals)
}

func clampFloat(value, minValue, maxValue float64) float64 {
	return math.Max(minValue, math.Min(maxValue, value))
}

func mapParticleBlend(b uint8) components.ParticleFilterMode {
	switch b {
	case 1:
		return components.PFilterAlphaKey
	case 3, 4, 7:
		return components.PFilterAdditive
	case 5:
		return components.PFilterModulate
	case 6:
		return components.PFilterModulate2x
	default:
		return components.PFilterBlend
	}
}

func decompressGravity(v []float64) any {
	if len(v) == 0 {
		return float64(0)
	}
	u := uint32(v[0])
	f := math.Float32frombits(u)
	if !math.IsInf(float64(f), 0) && !math.IsNaN(float64(f)) && math.Abs(float64(f)) < 1e4 {
		return 0.5 * float64(f)
	}
	bx := int(u & 0xFF)
	by := int((u >> 8) & 0xFF)
	bz := int((u >> 16) & 0xFFFF)
	if bx&0x80 != 0 {
		bx -= 0x100
	}
	if by&0x80 != 0 {
		by -= 0x100
	}
	if bz&0x8000 != 0 {
		bz -= 0x10000
	}
	dir := imath.Vector3{float64(bx) / 128, float64(by) / 128, 0}
	dot := dir[0]*dir[0] + dir[1]*dir[1]
	z := math.Sqrt(math.Max(0, 1-dot))
	mag := float64(bz) * 0.04238648
	if mag < 0 {
		z = -z
		mag = -mag
	}
	return -0.5 * z * mag
}

func partTrackTimes(p m2.PartTrack) []uint16 {
	seen := map[uint16]struct{}{}
	for _, t := range p.Timestamps {
		seen[t] = struct{}{}
	}
	out := make([]uint16, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func partValueAt(track m2.PartTrack, time uint16) []float64 {
	for i, t := range track.Timestamps {
		if t >= time && i < len(track.Values) {
			return track.Values[i]
		}
	}
	return nil
}

func particleTimeMiddle(p m2.ParticleEmitterEntry) float64 {
	mid, end, ok := particleAlphaMidEnd(p)
	if !ok || end == 0 {
		return 0
	}
	return float64(mid) / float64(end)
}

func particleAlphaMidEnd(p m2.ParticleEmitterEntry) (uint16, uint16, bool) {
	times := partTrackTimes(p.AlphaTrack)
	if len(times) == 0 {
		return 0, 0, false
	}
	return times[len(times)/2], times[len(times)-1], true
}

func particleMidEndOrZero(p m2.ParticleEmitterEntry) (uint16, uint16) {
	mid, end, ok := particleAlphaMidEnd(p)
	if !ok {
		return 0, 0
	}
	return mid, end
}

func particleMidOrZero(p m2.ParticleEmitterEntry) uint16 {
	mid, _, ok := particleAlphaMidEnd(p)
	if !ok {
		return 0
	}
	return mid
}

func particleEndOrZero(p m2.ParticleEmitterEntry) uint16 {
	_, end, ok := particleAlphaMidEnd(p)
	if !ok {
		return 0
	}
	if end == 0 {
		return 0
	}
	return end
}

func particleSegmentColors(p m2.ParticleEmitterEntry) [3]imath.Vector3 {
	mid, end := particleMidEndOrZero(p)
	v0 := partValueAt(p.ColorTrack, 0)
	if v0 == nil {
		v0 = []float64{255, 255, 255}
	}
	v1 := partValueAt(p.ColorTrack, mid)
	if v1 == nil {
		v1 = v0
	}
	v2 := partValueAt(p.ColorTrack, end)
	if v2 == nil {
		v2 = v1
	}
	conv := func(v []float64) imath.Vector3 {
		if len(v) < 3 {
			return imath.Vector3{1, 1, 1}
		}
		return imath.Vector3{v[2] / 255, v[1] / 255, v[0] / 255}
	}
	return [3]imath.Vector3{conv(v0), conv(v1), conv(v2)}
}

func particleSegmentAlphas(p m2.ParticleEmitterEntry) [3]float64 {
	mid, end := particleMidEndOrZero(p)
	conv := func(v []float64) float64 {
		if len(v) == 0 {
			return 255
		}
		return math.Round(255 * v[0] / 32767)
	}
	v0 := partValueAt(p.AlphaTrack, 0)
	if v0 == nil {
		v0 = []float64{32767}
	}
	v1 := partValueAt(p.AlphaTrack, mid)
	if v1 == nil {
		v1 = v0
	}
	v2 := partValueAt(p.AlphaTrack, end)
	if v2 == nil {
		v2 = v1
	}
	return [3]float64{conv(v0), conv(v1), conv(v2)}
}

func particleSegmentScaling(p m2.ParticleEmitterEntry) [3]float64 {
	mid, end := particleMidEndOrZero(p)
	factor := float64(p.TwinkleScale.Min+p.TwinkleScale.Max) / 2
	conv := func(v []float64) float64 {
		if len(v) < 2 {
			return factor
		}
		return math.Min(v[0], v[1]) * float64(factor)
	}
	v0 := partValueAt(p.ScaleTrack, 0)
	if v0 == nil {
		v0 = []float64{1, 1}
	}
	v1 := partValueAt(p.ScaleTrack, mid)
	if v1 == nil {
		v1 = v0
	}
	v2 := partValueAt(p.ScaleTrack, end)
	if v2 == nil {
		v2 = v1
	}
	return [3]float64{conv(v0), conv(v1), conv(v2)}
}

func particleHeadIntervals(p m2.ParticleEmitterEntry) [3]float64 {
	mid := particleMidOrZero(p)
	v0 := partValueAt(p.HeadCellTrack, 0)
	if v0 == nil {
		v0 = []float64{0}
	}
	v1 := partValueAt(p.HeadCellTrack, mid)
	if v1 == nil {
		v1 = v0
	}
	return [3]float64{v0[0], v1[0], 1}
}

func particleDecayIntervals(p m2.ParticleEmitterEntry) [3]float64 {
	mid, end := particleMidEndOrZero(p)
	v1 := partValueAt(p.HeadCellTrack, mid)
	if v1 == nil {
		v1 = []float64{0}
	}
	v2 := partValueAt(p.HeadCellTrack, end)
	if v2 == nil {
		v2 = v1
	}
	return [3]float64{v1[0], v2[0], 1}
}

func particleTailIntervals(p m2.ParticleEmitterEntry) [3]float64 {
	mid := particleMidOrZero(p)
	v0 := partValueAt(p.TailCellTrack, 0)
	if v0 == nil {
		v0 = []float64{0}
	}
	v1 := partValueAt(p.TailCellTrack, mid)
	if v1 == nil {
		v1 = v0
	}
	return [3]float64{v0[0], v1[0], 1}
}

func particleTailDecayIntervals(p m2.ParticleEmitterEntry) [3]float64 {
	mid, end := particleMidEndOrZero(p)
	v1 := partValueAt(p.TailCellTrack, mid)
	if v1 == nil {
		v1 = []float64{0}
	}
	v2 := partValueAt(p.TailCellTrack, end)
	if v2 == nil {
		v2 = v1
	}
	return [3]float64{v1[0], v2[0], 1}
}

func correctParticleEmission(p m2.ParticleEmitterEntry, baseEmissionRate components.AnimatedOrStatic[float64]) (lifeSpan, tailLength float64, emissionRate components.AnimatedOrStatic[float64]) {
	lifeSpan = maxTrackFloat(p.Lifespan)
	if lifeSpan <= 0 {
		lifeSpan = 1
	}
	tailLengthWowMax := math.Min(float64(p.TailLength), lifeSpan)
	wowClampsTailToAge := p.Flags&0x400 != 0
	tailLengthExpected := tailLengthWowMax
	if wowClampsTailToAge {
		if tailLengthWowMax < lifeSpan {
			tailLengthExpected = tailLengthWowMax - (tailLengthWowMax*tailLengthWowMax)/(2*lifeSpan)
		} else {
			tailLengthExpected = lifeSpan * 0.5
		}
	}
	alphaWeightFactor := particleAlphaWeightFactor(p)
	tailLengthVisible := tailLengthExpected * alphaWeightFactor
	emissionRateMax := maxTrackFloat(p.EmissionRate)
	coverageMinTime := 0.0
	if emissionRateMax > 0 {
		coverageMinTime = 1.0 / emissionRateMax
	}
	tailLength = math.Min(tailLengthWowMax, math.Max(tailLengthVisible, coverageMinTime))
	coverageMinRate := 0.0
	if tailLength > 0 {
		coverageMinRate = 1.0 / tailLength
	}
	shortenedTail := tailLength+1e-6 < float64(p.TailLength)
	needsRateFloor := (wowClampsTailToAge || shortenedTail) && emissionRateMax+1e-6 < coverageMinRate
	alphaEmissionBoost := 1.0
	if needsRateFloor && alphaWeightFactor < 0.7 {
		alphaEmissionBoost = 1.0 / math.Max(0.5, alphaWeightFactor)
	}
	emissionRate = baseEmissionRate
	if needsRateFloor {
		applyAnimatedOrStaticFloat(&emissionRate, func(v float64) float64 {
			return math.Max(v, coverageMinRate) * alphaEmissionBoost
		})
	}
	return lifeSpan, tailLength, emissionRate
}

func particleAlphaWeightFactor(p m2.ParticleEmitterEntry) float64 {
	if len(p.AlphaTrack.Values) == 0 {
		return 1
	}
	sum := 0.0
	count := 0
	for _, values := range p.AlphaTrack.Values {
		if len(values) == 0 {
			continue
		}
		sum += math.Max(0, math.Min(1, values[0]/32767))
		count++
	}
	if count == 0 {
		return 1
	}
	return sum / float64(count)
}

func applyParticleDrag(speed *components.AnimatedOrStatic[float64], lifeSpan, drag float64) {
	applyAnimatedOrStaticFloat(speed, func(v float64) float64 {
		return calculateEquivalentVelocityNoDrag(v, lifeSpan, drag)
	})
}

func applyAnimatedOrStaticFloat(value *components.AnimatedOrStatic[float64], fn func(float64) float64) {
	if value == nil {
		return
	}
	if value.Static {
		value.Value = fn(value.Value)
		return
	}
	if value.Anim == nil {
		return
	}
	for t, raw := range value.Anim.KeyFrames {
		if v, ok := raw.(float64); ok {
			value.Anim.KeyFrames[t] = fn(v)
		}
	}
}

func calculateEquivalentVelocityNoDrag(initialVelocity, lifetime, drag float64) float64 {
	if drag == 0 {
		return initialVelocity
	}
	const deltaTime = 1.0 / 30.0
	decayFactor := math.Max(0, 1-drag*deltaTime)
	steps := lifetime / deltaTime
	integral := initialVelocity * (math.Pow(decayFactor, steps) - 1) / math.Log(decayFactor) * deltaTime
	result := integral / lifetime
	if math.IsNaN(result) {
		return initialVelocity
	}
	return result
}

func maxTrackFloat(t m2.Track) float64 {
	max := 0.0
	for _, row := range t.Values {
		for _, vals := range row {
			if len(vals) > 0 && vals[0] > max {
				max = vals[0]
			}
		}
	}
	return max
}

func mdlTextureKey(tex components.Texture) string {
	replaceableID := 0
	hasReplaceableID := false
	if tex.ReplaceableID != nil {
		replaceableID = *tex.ReplaceableID
		hasReplaceableID = true
	}
	return fmt.Sprintf("%t|%d|%s|%t|%t|%d|%s", hasReplaceableID, replaceableID, tex.Image, tex.WrapWidth, tex.WrapHeight, tex.WowData.Type, tex.WowData.PngPath)
}

func storeMdlTexture(m *mdl.MDL, tex *components.Texture) *components.Texture {
	if tex == nil || m == nil {
		return tex
	}
	key := mdlTextureKey(*tex)
	for _, existing := range m.Textures {
		if mdlTextureKey(*existing) == key {
			return existing
		}
	}
	cloned := *tex
	m.Textures = append(m.Textures, &cloned)
	return m.Textures[len(m.Textures)-1]
}
