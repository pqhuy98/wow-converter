package mdl

import (
	"fmt"
	stdmath "math"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

type Version struct {
	FormatVersion int
}

type ModelInfo struct {
	components.Bound
	Name      string
	BlendTime int
}

type MDL struct {
	Version           Version
	Model             ModelInfo
	GlobalSequences   []*components.GlobalSequence
	Sequences         []components.Sequence
	Textures          []*components.Texture
	Materials         []*components.Material
	TextureAnims      []components.TextureAnim
	Geosets           []*components.Geoset
	GeosetAnims       []components.GeosetAnim
	Bones             []*components.Bone
	Attachments       []*components.AttachmentPoint
	Lights            []*components.Light
	ParticleEmitter2s []*components.ParticleEmitter2
	RibbonEmitters    []*components.RibbonEmitter
	Cameras           []components.Camera
	EventObjects      []*components.EventObject
	Helpers           []*components.Helper
	CollisionShapes   []*components.CollisionShape
	Modify            *Modify
	BoundsOverriden   func(obj *components.Bound)
	WowAttachments    []components.WowAttachment
	AccumScale        float64
}

type NewMDLOptions struct {
	FormatVersion int
	Name          string
}

func New(opts NewMDLOptions) *MDL {
	m := &MDL{
		Version: Version{FormatVersion: opts.FormatVersion},
		Model: ModelInfo{
			Name:      opts.Name,
			BlendTime: 150,
		},
		AccumScale: 1,
	}
	m.Modify = NewModify(m)
	return m
}

func (m *MDL) GetNodes() []components.Node {
	nodes := make([]components.Node, 0)
	for _, b := range m.Bones {
		nodes = append(nodes, b)
	}
	for _, a := range m.Attachments {
		nodes = append(nodes, a)
	}
	for _, l := range m.Lights {
		nodes = append(nodes, l)
	}
	for _, r := range m.RibbonEmitters {
		nodes = append(nodes, r)
	}
	for _, p := range m.ParticleEmitter2s {
		nodes = append(nodes, p)
	}
	for _, e := range m.EventObjects {
		nodes = append(nodes, e)
	}
	for _, c := range m.CollisionShapes {
		nodes = append(nodes, c)
	}
	for _, h := range m.Helpers {
		nodes = append(nodes, h)
	}
	return nodes
}

func (m *MDL) GetAnimated() []*components.Animation {
	seen := map[*components.Animation]struct{}{}
	var result []*components.Animation
	add := func(anim *components.Animation) {
		if anim == nil || len(anim.KeyFrames) == 0 {
			return
		}
		if _, ok := seen[anim]; ok {
			return
		}
		seen[anim] = struct{}{}
		result = append(result, anim)
	}
	addAnimOrStatic := func(v any) {
		add(components.GetAnim(v))
	}

	for _, node := range m.GetNodes() {
		add(node.NodeTranslation())
		add(node.NodeRotation())
		add(node.NodeScaling())
	}
	for _, cam := range m.Cameras {
		add(cam.Translation)
		add(cam.Rotation)
		add(cam.Scaling)
	}
	for _, texAnim := range m.TextureAnims {
		add(texAnim.Translation)
		add(texAnim.Rotation)
		add(texAnim.Scaling)
	}
	for _, mat := range m.Materials {
		for _, layer := range mat.Layers {
			addAnimOrStatic(layer.Alpha)
			if layer.TVertexAnim != nil {
				add(layer.TVertexAnim.Translation)
				add(layer.TVertexAnim.Rotation)
				add(layer.TVertexAnim.Scaling)
			}
		}
	}
	for _, geosetAnim := range m.GeosetAnims {
		addAnimOrStatic(geosetAnim.Alpha)
		addAnimOrStatic(geosetAnim.Color)
	}
	for _, l := range m.Lights {
		add(l.Visibility)
		addAnimOrStatic(l.AttenuationStart)
		addAnimOrStatic(l.AttenuationEnd)
		addAnimOrStatic(l.Intensity)
		addAnimOrStatic(l.Color)
		addAnimOrStatic(l.AmbientIntensity)
		addAnimOrStatic(l.AmbientColor)
	}
	for _, r := range m.RibbonEmitters {
		add(r.Visibility)
		if r.HeightAbove != nil {
			addAnimOrStatic(r.HeightAbove)
		}
		if r.HeightBelow != nil {
			addAnimOrStatic(r.HeightBelow)
		}
		if r.Alpha != nil {
			addAnimOrStatic(r.Alpha)
		}
		if r.Color != nil {
			addAnimOrStatic(r.Color)
		}
		if r.TextureSlot != nil {
			addAnimOrStatic(r.TextureSlot)
		}
	}
	for _, p := range m.ParticleEmitter2s {
		add(p.Visibility)
		addAnimOrStatic(p.EmissionRate)
		addAnimOrStatic(p.Latitude)
		addAnimOrStatic(p.Speed)
		addAnimOrStatic(p.Variation)
		addAnimOrStatic(p.Gravity)
		addAnimOrStatic(p.Width)
		addAnimOrStatic(p.Length)
	}
	return result
}

func (m *MDL) UpdateIDs() {
	for i, gs := range m.GlobalSequences {
		gs.ID = i
	}
	for i := range m.Textures {
		m.Textures[i].ID = i
	}
	textureIDs := map[string]int{}
	for i := range m.Textures {
		key := textureUpdateKey(*m.Textures[i])
		if _, ok := textureIDs[key]; !ok {
			textureIDs[key] = i
		}
	}
	syncTextureID := func(texture *components.Texture) {
		if texture == nil {
			return
		}
		if id, ok := textureIDs[textureUpdateKey(*texture)]; ok {
			texture.ID = id
		}
	}
	for i := range m.Materials {
		m.Materials[i].ID = i
		for li := range m.Materials[i].Layers {
			syncTextureID(m.Materials[i].Layers[li].Texture)
		}
	}
	for _, emitter := range m.ParticleEmitter2s {
		syncTextureID(emitter.Texture)
	}
	for i := range m.TextureAnims {
		m.TextureAnims[i].ID = i
	}
	for i := range m.GeosetAnims {
		m.GeosetAnims[i].ID = i
	}
	for i, geoset := range m.Geosets {
		geoset.ID = i
	}
	nodes := m.GetNodes()
	for i, node := range nodes {
		node.SetNodeObjectID(i)
	}
	for i := range m.Attachments {
		m.Attachments[i].AttachmentID = i
	}
}

func textureUpdateKey(texture components.Texture) string {
	replaceableID := 0
	hasReplaceableID := false
	if texture.ReplaceableID != nil {
		replaceableID = *texture.ReplaceableID
		hasReplaceableID = true
	}
	return fmt.Sprintf(
		"%t|%d|%s|%t|%t|%d|%s",
		hasReplaceableID,
		replaceableID,
		texture.Image,
		texture.WrapWidth,
		texture.WrapHeight,
		texture.WowData.Type,
		texture.WowData.PngPath,
	)
}

func (m *MDL) ToMdl() string {
	return m.formatString()
}

func (m *MDL) ToMdx() ([]byte, error) {
	return mdx.ConvertMdlToMdx(m.ToMdl())
}

func (m *MDL) Sync() {
	m.SyncExtents()

	updateGlobalSeq := func(globalSeq *components.GlobalSequence, values ...int) {
		if globalSeq == nil {
			return
		}
		for _, v := range values {
			if v > globalSeq.Duration {
				globalSeq.Duration = v
			}
		}
	}

	for _, texAnim := range m.TextureAnims {
		if texAnim.Translation != nil {
			updateGlobalSeq(texAnim.Translation.GlobalSeq, components.SortedKeyInts(texAnim.Translation.KeyFrames)...)
		}
		if texAnim.Rotation != nil {
			updateGlobalSeq(texAnim.Rotation.GlobalSeq, components.SortedKeyInts(texAnim.Rotation.KeyFrames)...)
		}
		if texAnim.Scaling != nil {
			updateGlobalSeq(texAnim.Scaling.GlobalSeq, components.SortedKeyInts(texAnim.Scaling.KeyFrames)...)
		}
	}
	for _, bone := range m.Bones {
		if bone.Translation != nil {
			updateGlobalSeq(bone.Translation.GlobalSeq, components.SortedKeyInts(bone.Translation.KeyFrames)...)
		}
		if bone.Rotation != nil {
			updateGlobalSeq(bone.Rotation.GlobalSeq, components.SortedKeyInts(bone.Rotation.KeyFrames)...)
		}
		if bone.Scaling != nil {
			updateGlobalSeq(bone.Scaling.GlobalSeq, components.SortedKeyInts(bone.Scaling.KeyFrames)...)
		}
	}

	geosetsPerBone := map[*components.Bone]map[*components.Geoset]struct{}{}
	boneChildren := map[*components.Bone][]*components.Bone{}

	for _, geoset := range m.Geosets {
		bones := map[*components.Bone]struct{}{}
		for _, matrix := range geoset.Matrices {
			for _, b := range matrix.Bones {
				bones[b] = struct{}{}
			}
		}
		for _, v := range geoset.Vertices {
			for _, w := range v.SkinWeights {
				bones[w.Bone] = struct{}{}
			}
		}
		for bone := range bones {
			if geosetsPerBone[bone] == nil {
				geosetsPerBone[bone] = map[*components.Geoset]struct{}{}
			}
			geosetsPerBone[bone][geoset] = struct{}{}
		}
	}

	for _, bone := range m.Bones {
		if geosets, ok := geosetsPerBone[bone]; ok {
			if len(geosets) > 1 {
				bone.GeosetMulti = true
				bone.Geoset = nil
			} else {
				for g := range geosets {
					bone.Geoset = g
					bone.GeosetMulti = false
				}
			}
		}
		if bone.ParentBone != nil {
			boneChildren[bone.ParentBone] = append(boneChildren[bone.ParentBone], bone)
		} else if bone.Parent != nil {
			if pb, ok := bone.Parent.(*components.Bone); ok {
				boneChildren[pb] = append(boneChildren[pb], bone)
			}
		}
	}

	var dfs func(node *components.Bone)
	dfs = func(node *components.Bone) {
		for _, child := range boneChildren[node] {
			dfs(child)
			if child.GeosetMulti {
				node.GeosetMulti = true
				node.Geoset = nil
			}
			if child.Geoset != nil && node.Geoset != nil && child.Geoset != node.Geoset {
				node.GeosetMulti = true
				node.Geoset = nil
			}
			if child.Geoset != nil && node.Geoset == nil && !node.GeosetMulti {
				node.Geoset = child.Geoset
			}
		}
	}
	for _, bone := range m.Bones {
		if bone.ParentBone == nil && bone.Parent == nil {
			dfs(bone)
		}
	}

	needsMaterial := false
	for _, geoset := range m.Geosets {
		if geoset.Material == nil {
			needsMaterial = true
			break
		}
	}
	if needsMaterial {
		if len(m.Materials) == 0 {
			tex := &components.Texture{
				ID:      0,
				Image:   "",
				WowData: components.TextureWowData{Type: 0, PngPath: ""},
			}
			m.Textures = append(m.Textures, tex)
			m.Materials = []*components.Material{{
				ID:            0,
				TwoSided:      false,
				ConstantColor: false,
				Layers: []components.Layer{{
					FilterMode: components.BlendNone,
					Texture:    tex,
					Unshaded:   false, SphereEnvMap: false, TwoSided: false,
					Unfogged: false, Unlit: false, NoDepthTest: false, NoDepthSet: false,
					Alpha: components.AnimatedOrStatic[float64]{Static: true, Value: 1},
				}},
			}}
		}
		for _, geoset := range m.Geosets {
			if geoset.Material == nil {
				geoset.Material = m.Materials[0]
			}
		}
	}
	m.UpdateIDs()
}

func (m *MDL) SyncExtents() {
	var filtered []*components.Geoset
	for _, geoset := range m.Geosets {
		if len(geoset.Vertices) > 0 && len(geoset.Faces) > 0 {
			filtered = append(filtered, geoset)
		}
	}
	m.Geosets = filtered

	for _, geoset := range m.Geosets {
		min := imath.Vector3{stdmath.Inf(1), stdmath.Inf(1), stdmath.Inf(1)}
		max := imath.Vector3{stdmath.Inf(-1), stdmath.Inf(-1), stdmath.Inf(-1)}
		for _, v := range geoset.Vertices {
			x, y, z := v.Position[0], v.Position[1], v.Position[2]
			if x < min[0] {
				min[0] = x
			}
			if y < min[1] {
				min[1] = y
			}
			if z < min[2] {
				min[2] = z
			}
			if x > max[0] {
				max[0] = x
			}
			if y > max[1] {
				max[1] = y
			}
			if z > max[2] {
				max[2] = z
			}
		}
		geoset.MinimumExtent = min
		geoset.MaximumExtent = max
		positions := make([]imath.Vector3, len(geoset.Vertices))
		for i, v := range geoset.Vertices {
			positions[i] = v.Position
		}
		geoset.BoundsRadius = calculateBoundRadius(positions)
	}

	if len(m.Geosets) > 0 {
		modelMin := m.Geosets[0].MinimumExtent
		modelMax := m.Geosets[0].MaximumExtent
		maxRadius := m.Geosets[0].BoundsRadius
		for _, geoset := range m.Geosets[1:] {
			modelMin = imath.V3Min(modelMin, geoset.MinimumExtent)
			modelMax = imath.V3Max(modelMax, geoset.MaximumExtent)
			if geoset.BoundsRadius > maxRadius {
				maxRadius = geoset.BoundsRadius
			}
		}
		m.Model.MinimumExtent = modelMin
		m.Model.MaximumExtent = modelMax
		m.Model.BoundsRadius = maxRadius
	}

	for i := range m.Sequences {
		m.Sequences[i].MinimumExtent = components.CopyVector3(m.Model.MinimumExtent)
		m.Sequences[i].MaximumExtent = components.CopyVector3(m.Model.MaximumExtent)
		m.Sequences[i].BoundsRadius = m.Model.BoundsRadius
	}
}

func (m *MDL) formatString() string {
	m.UpdateIDs()

	if m.BoundsOverriden != nil {
		m.BoundsOverriden(&m.Model.Bound)
		for _, geoset := range m.Geosets {
			m.BoundsOverriden(&geoset.Bound)
		}
		for i := range m.Sequences {
			m.BoundsOverriden(&m.Sequences[i].Bound)
		}
	}

	result := "// Exported by Huy's wow-converter\n" +
		m.versionToString() + "\n" +
		m.modelToString() + "\n" +
		components.SequencesToString(m.Sequences) + "\n" +
		components.GlobalSequencesToString(m.GlobalSequences) + "\n" +
		components.TexturesToString(m.Textures) + "\n" +
		components.MaterialsToString(m.Version.FormatVersion, m.Materials) + "\n" +
		components.TextureAnimsToString(m.TextureAnims) + "\n" +
		components.GeosetsToString(m.Version.FormatVersion, m.Geosets, m.Bones, m.Sequences) + "\n" +
		components.GeosetAnimsToString(m.GeosetAnims) + "\n" +
		components.BonesToString(m.Bones) + "\n" +
		components.AttachmentPointsToString(m.Attachments) + "\n" +
		components.LightsToString(m.Lights) + "\n" +
		components.RibbonEmittersToString(m.RibbonEmitters) + "\n" +
		components.ParticleEmitter2sToString(m.ParticleEmitter2s) + "\n" +
		components.CamerasToString(m.Cameras) + "\n" +
		components.EventObjectsToString(m.EventObjects) + "\n" +
		components.CollisionShapesToString(m.CollisionShapes) + "\n" +
		components.HelpersToString(m.Helpers) + "\n" +
		components.PivotPointsToString(m.GetNodes())

	result = indentMDLLikeTS(result)

	if m.BoundsOverriden != nil {
		m.SyncExtents()
	}
	return result
}

func (m *MDL) versionToString() string {
	return "Version {\nFormatVersion " + components.FVal(float64(m.Version.FormatVersion)) + ",\n}"
}

func (m *MDL) modelToString() string {
	return "Model \"" + m.Model.Name + "\" {\n" +
		"NumGeosets " + components.FVal(float64(len(m.Geosets))) + ",\n" +
		"NumBones " + components.FVal(float64(len(m.Bones))) + ",\n" +
		"NumAttachments " + components.FVal(float64(len(m.Attachments))) + ",\n" +
		"BlendTime " + components.FVal(float64(m.Model.BlendTime)) + ",\n" +
		"MinimumExtent { " + components.FVector3(m.Model.MinimumExtent) + " },\n" +
		"MaximumExtent { " + components.FVector3(m.Model.MaximumExtent) + " },\n" +
		"BoundsRadius " + components.FVal(m.Model.BoundsRadius) + ",\n" +
		"}"
}

func indentMDLLikeTS(input string) string {
	lines := strings.Split(input, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		trimmed = append(trimmed, l)
	}
	depth := 0
	out := make([]string, 0, len(trimmed))
	for _, l := range trimmed {
		if l == "}" || l == "}," {
			depth--
		}
		out = append(out, strings.Repeat("\t", depth)+l)
		if strings.HasSuffix(l, "{") {
			depth++
		}
	}
	return strings.Join(out, "\n")
}

func calculateBoundRadius(vertices []imath.Vector3) float64 {
	maxDistance := 0.0
	for _, v := range vertices {
		distance := stdmath.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
		if distance > maxDistance {
			maxDistance = distance
		}
	}
	return maxDistance
}
