package mdl

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func (mod *Modify) RemoveUnusedMaterialsTextures() *Modify {
	textureKey := func(tex components.Texture) string {
		replaceableID := 0
		hasReplaceableID := false
		if tex.ReplaceableID != nil {
			replaceableID = *tex.ReplaceableID
			hasReplaceableID = true
		}
		return fmt.Sprintf("%t|%d|%s|%t|%t|%d|%s", hasReplaceableID, replaceableID, tex.Image, tex.WrapWidth, tex.WrapHeight, tex.WowData.Type, tex.WowData.PngPath)
	}
	usedTextures := map[string]*components.Texture{}
	var textureOrder []string
	getTexture := func(tex *components.Texture) *components.Texture {
		key := textureKey(*tex)
		if existing, ok := usedTextures[key]; ok {
			return existing
		}
		usedTextures[key] = tex
		textureOrder = append(textureOrder, key)
		return tex
	}

	seenGeosetMat := map[*components.Material]struct{}{}
	var geosetMaterials []*components.Material
	for _, geoset := range mod.MDL.Geosets {
		if geoset.Material == nil {
			continue
		}
		if _, ok := seenGeosetMat[geoset.Material]; ok {
			continue
		}
		seenGeosetMat[geoset.Material] = struct{}{}
		geosetMaterials = append(geosetMaterials, geoset.Material)
	}

	for i := range mod.MDL.TextureAnims {
		mod.MDL.TextureAnims[i].ID = i
	}
	textureAnimKey := func(texAnim components.TextureAnim) string {
		ta := texAnim
		ta.ID = 0
		b, _ := json.Marshal(ta)
		return string(b)
	}
	textureAnimByKey := map[string]*components.TextureAnim{}
	for i := range mod.MDL.TextureAnims {
		texAnim := &mod.MDL.TextureAnims[i]
		key := textureAnimKey(*texAnim)
		if _, ok := textureAnimByKey[key]; !ok {
			textureAnimByKey[key] = texAnim
		}
	}
	canonicalTextureAnim := func(texAnim **components.TextureAnim) {
		if texAnim == nil || *texAnim == nil {
			return
		}
		if canonical, ok := textureAnimByKey[textureAnimKey(**texAnim)]; ok {
			*texAnim = canonical
		}
	}

	for i, mat := range geosetMaterials {
		for li := range mat.Layers {
			layer := &mat.Layers[li]
			if layer.Texture == nil {
				continue
			}
			if layer.Texture.Image == "" {
				log.Printf("%s", ansi.Redf("Empty texture, i: %d, wow type: %d", i, layer.Texture.WowData.Type))
			}
			layer.Texture = getTexture(layer.Texture)
			canonicalTextureAnim(&layer.TVertexAnim)
		}
	}
	for _, e := range mod.MDL.ParticleEmitter2s {
		if e.Texture != nil {
			e.Texture = getTexture(e.Texture)
		}
	}

	textures := make([]*components.Texture, 0, len(textureOrder))
	for _, key := range textureOrder {
		textures = append(textures, usedTextures[key])
	}
	mod.MDL.Textures = textures

	materialKey := func(mat components.Material) string {
		m := mat
		m.ID = 0
		b, _ := json.Marshal(m)
		return string(b)
	}
	usedMaterials := map[string]*components.Material{}
	var materialOrder []string
	for _, geoset := range mod.MDL.Geosets {
		if geoset.Material == nil {
			continue
		}
		key := materialKey(*geoset.Material)
		if existing, ok := usedMaterials[key]; ok {
			geoset.Material = existing
			continue
		}
		usedMaterials[key] = geoset.Material
		materialOrder = append(materialOrder, key)
	}
	materials := make([]*components.Material, 0, len(materialOrder))
	for _, key := range materialOrder {
		materials = append(materials, usedMaterials[key])
	}
	mod.MDL.Materials = materials

	return mod
}

func (mod *Modify) RemoveUnusedNodes() *Modify {
	usedNodes := map[components.Node]struct{}{}
	for _, n := range mod.MDL.Attachments {
		usedNodes[n] = struct{}{}
	}
	for _, n := range mod.MDL.ParticleEmitter2s {
		usedNodes[n] = struct{}{}
	}
	for _, n := range mod.MDL.Lights {
		usedNodes[n] = struct{}{}
	}
	for _, n := range mod.MDL.RibbonEmitters {
		usedNodes[n] = struct{}{}
	}
	for _, n := range mod.MDL.Helpers {
		usedNodes[n] = struct{}{}
	}
	for _, n := range mod.MDL.EventObjects {
		usedNodes[n] = struct{}{}
	}
	for _, n := range mod.MDL.CollisionShapes {
		usedNodes[n] = struct{}{}
	}
	for _, geoset := range mod.MDL.Geosets {
		for _, v := range geoset.Vertices {
			for _, sw := range v.SkinWeights {
				usedNodes[sw.Bone] = struct{}{}
			}
			if v.Matrix != nil {
				for _, b := range v.Matrix.Bones {
					usedNodes[b] = struct{}{}
				}
			}
		}
	}

	childrenList := BuildChildrenLists(mod.MDL)
	var dfs func(cur components.Node) bool
	dfs = func(cur components.Node) bool {
		isUsed := false
		if _, ok := usedNodes[cur]; ok {
			isUsed = true
		}
		for _, child := range childrenList[cur] {
			if dfs(child) {
				isUsed = true
			}
		}
		if isUsed {
			usedNodes[cur] = struct{}{}
		}
		return isUsed
	}
	for _, b := range mod.MDL.Bones {
		if b.NodeParent() == nil {
			dfs(b)
		}
	}

	var filtered []*components.Bone
	for _, b := range mod.MDL.Bones {
		if _, ok := usedNodes[b]; ok {
			filtered = append(filtered, b)
		}
	}
	mod.MDL.Bones = filtered
	return mod
}

func (mod *Modify) RemoveUnusedVertices() *Modify {
	for _, geoset := range mod.MDL.Geosets {
		used := map[*components.GeosetVertex]struct{}{}
		for _, face := range geoset.Faces {
			for _, v := range face.Vertices {
				used[v] = struct{}{}
			}
		}
		var filtered []*components.GeosetVertex
		for _, v := range geoset.Vertices {
			if _, ok := used[v]; ok {
				filtered = append(filtered, v)
			}
		}
		geoset.Vertices = filtered
	}
	mod.MDL.SyncExtents()
	return mod
}

func (mod *Modify) RemoveCinematicSequences() *Modify {
	var filtered []components.Sequence
	for _, seq := range mod.MDL.Sequences {
		if !contains(seq.Name, "Cinematic") || seq.Keep {
			filtered = append(filtered, seq)
		}
	}
	mod.MDL.Sequences = filtered
	return mod
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func (mod *Modify) OptimizeKeyFrames() *Modify {
	seqIntervals := make([][2]int, len(mod.MDL.Sequences))
	for i, s := range mod.MDL.Sequences {
		seqIntervals[i] = [2]int{s.Interval[0], s.Interval[1]}
	}
	sort.Slice(seqIntervals, func(i, j int) bool { return seqIntervals[i][0] < seqIntervals[j][0] })

	inSequence := func(anim *components.Animation, timestamp int, cursor *int) bool {
		if anim.GlobalSeq != nil {
			return timestamp < anim.GlobalSeq.Duration
		}
		i := *cursor
		for i < len(seqIntervals) && seqIntervals[i][1] < timestamp {
			i++
		}
		*cursor = i
		return i < len(seqIntervals) && seqIntervals[i][0] <= timestamp
	}

	thresholds := map[components.AnimationType]float64{
		components.AnimTypeTranslation: 0.005,
		components.AnimTypeRotation:    0.001,
		components.AnimTypeScaling:     0.01,
		components.AnimTypeAlpha:       0.01,
		components.AnimTypeColor:       0.01,
		components.AnimTypeTVertex:     0.01,
		components.AnimTypeTVertexAnim: 0.01,
	}

	diffBetween := func(a, b any) float64 {
		switch av := a.(type) {
		case float64:
			if bv, ok := b.(float64); ok {
				return math.Abs(av - bv)
			}
		case int:
			if bv, ok := b.(int); ok {
				return math.Abs(float64(av - bv))
			}
		case imath.Vector3:
			if bv, ok := b.(imath.Vector3); ok {
				return math.Abs(av[0]-bv[0]) + math.Abs(av[1]-bv[1]) + math.Abs(av[2]-bv[2])
			}
		case imath.QuaternionRotation:
			if bv, ok := b.(imath.QuaternionRotation); ok {
				return math.Abs(av[0]-bv[0]) + math.Abs(av[1]-bv[1]) + math.Abs(av[2]-bv[2]) + math.Abs(av[3]-bv[3])
			}
		case []float64:
			if bv, ok := b.([]float64); ok && len(av) == len(bv) {
				acc := 0.0
				for i := range av {
					acc += math.Abs(av[i] - bv[i])
				}
				return acc
			}
		}
		return math.Inf(1)
	}

	optimiseAnim := func(anim *components.Animation, threshold float64) {
		if anim == nil || len(anim.KeyFrames) <= 2 {
			return
		}
		times := components.SortedKeyInts(anim.KeyFrames)
		t0 := times[0]
		cursor := 0
		for k := 1; k < len(times); k++ {
			v0 := anim.KeyFrames[t0]
			t1 := times[k]
			v1 := anim.KeyFrames[t1]
			var t2 int
			var v2 any
			hasV2 := false
			if k+1 < len(times) {
				t2 = times[k+1]
				v2 = anim.KeyFrames[t2]
				hasV2 = true
			}

			inside := inSequence(anim, t1, &cursor)
			if !inside {
				delete(anim.KeyFrames, t1)
				continue
			}

			if k < len(times)-1 {
				if diffBetween(v0, v1) >= threshold {
					t0 = t1
					continue
				}
			}

			firstFrame := false
			for _, interval := range seqIntervals {
				sStart := interval[0]
				if t0 < sStart && sStart <= t1 {
					firstFrame = true
					break
				}
			}

			lastFrame := k == len(times)-1
			if !lastFrame {
				nextT := times[k+1]
				for _, interval := range seqIntervals {
					sEnd := interval[1]
					if t1 <= sEnd && sEnd < nextT {
						lastFrame = true
						break
					}
				}
			}

			if (!inside || (!firstFrame && !lastFrame)) && hasV2 && isTruthyForTS(v2) && diffBetween(v0, v2) < threshold {
				delete(anim.KeyFrames, t1)
				continue
			}
			t0 = t1
		}

		cursor = 0
		if len(times) > 0 && !inSequence(anim, times[0], &cursor) {
			delete(anim.KeyFrames, times[0])
		}
	}

	mod.MDL.UpdateIDs()

	usedGlobalSequences := map[*components.GlobalSequence]struct{}{}
	for _, anim := range mod.MDL.GetAnimated() {
		if anim.GlobalSeq != nil {
			usedGlobalSequences[anim.GlobalSeq] = struct{}{}
		}
		threshold := thresholds[anim.Type]
		if threshold == 0 {
			threshold = 0.01
		}
		optimiseAnim(anim, threshold)
	}
	filterSortGlobalSequencesPreservePointers(mod.MDL, usedGlobalSequences)
	mod.MDL.UpdateIDs()

	neverVisible := func(visibility *components.Animation) bool {
		if visibility == nil || len(visibility.KeyFrames) == 0 {
			return false
		}
		for _, v := range visibility.KeyFrames {
			switch val := v.(type) {
			case float64:
				if val != 0 {
					return false
				}
			case int:
				if val != 0 {
					return false
				}
			default:
				return false
			}
		}
		return true
	}

	var pe2 []*components.ParticleEmitter2
	for _, e := range mod.MDL.ParticleEmitter2s {
		if !neverVisible(e.Visibility) {
			pe2 = append(pe2, e)
		}
	}
	mod.MDL.ParticleEmitter2s = pe2

	var ribbons []*components.RibbonEmitter
	for _, e := range mod.MDL.RibbonEmitters {
		if !neverVisible(e.Visibility) {
			ribbons = append(ribbons, e)
		}
	}
	mod.MDL.RibbonEmitters = ribbons

	var lights []*components.Light
	for _, e := range mod.MDL.Lights {
		if !neverVisible(e.Visibility) {
			lights = append(lights, e)
		}
	}
	mod.MDL.Lights = lights
	return mod
}

func isTruthyForTS(v any) bool {
	switch value := v.(type) {
	case nil:
		return false
	case bool:
		return value
	case float64:
		return value != 0 && !math.IsNaN(value)
	case float32:
		return value != 0 && !math.IsNaN(float64(value))
	case int:
		return value != 0
	case int32:
		return value != 0
	case int64:
		return value != 0
	case uint:
		return value != 0
	case uint32:
		return value != 0
	case uint64:
		return value != 0
	case string:
		return value != ""
	default:
		return true
	}
}

func filterSortGlobalSequencesPreservePointers(m *MDL, used map[*components.GlobalSequence]struct{}) {
	var filtered []*components.GlobalSequence
	for _, gs := range m.GlobalSequences {
		if gs == nil {
			continue
		}
		if _, ok := used[gs]; ok {
			filtered = append(filtered, gs)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Duration < filtered[j].Duration
	})
	m.GlobalSequences = filtered
}
