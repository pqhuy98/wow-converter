package mdx

import "strings"

// SaveMdl serializes decoded model fields as inspectable MDL text. Opaque binary
// chunks have no MDL representation; callers requiring them must use SaveMdx.
func (m *Model) SaveMdl() string {
	s := NewTokenStream("")
	s.writer = &strings.Builder{}
	s.StartBlock("Version")
	s.WriteNumberAttrib("FormatVersion", float64(m.Version))
	s.EndBlock()
	s.StartObjectBlock("Model", m.Name)
	s.WriteNumberAttrib("BlendTime", float64(m.BlendTime))
	m.Extent.WriteMdl(s)
	if m.AnimationFile != "" {
		s.WriteStringAttrib("AnimationFile", m.AnimationFile)
	}
	s.EndBlock()
	writeMdlObjects(s, "Sequences", m.Sequences, m.Version)
	if len(m.GlobalSequences) > 0 {
		s.StartBlock("GlobalSequences", len(m.GlobalSequences))
		for _, v := range m.GlobalSequences {
			s.WriteNumberAttrib("Duration", float64(v))
		}
		s.EndBlock()
	}
	writeMdlObjects(s, "Textures", m.Textures, m.Version)
	writeMdlObjects(s, "Materials", m.Materials, m.Version)
	writeMdlObjects(s, "TextureAnims", m.TextureAnimations, m.Version)
	writeMdlObjects(s, "", m.Geosets, m.Version)
	writeMdlObjects(s, "", m.GeosetAnimations, m.Version)
	writeMdlObjects(s, "", m.Bones, m.Version)
	writeMdlObjects(s, "", m.Lights, m.Version)
	writeMdlObjects(s, "", m.Helpers, m.Version)
	writeMdlObjects(s, "", m.Attachments, m.Version)
	if len(m.PivotPoints) > 0 {
		s.StartBlock("PivotPoints", len(m.PivotPoints))
		for _, v := range m.PivotPoints {
			s.WriteVector(v)
		}
		s.EndBlock()
	}
	writeMdlObjects(s, "", m.ParticleEmitters, m.Version)
	writeMdlObjects(s, "", m.ParticleEmitters2, m.Version)
	writeMdlObjects(s, "", m.ParticleEmittersPopcorn, m.Version)
	writeMdlObjects(s, "", m.RibbonEmitters, m.Version)
	writeMdlObjects(s, "", m.Cameras, m.Version)
	writeMdlObjects(s, "", m.EventObjects, m.Version)
	writeMdlObjects(s, "", m.CollisionShapes, m.Version)
	writeMdlObjects(s, "", m.FaceEffects, m.Version)
	if len(m.BindPose) > 0 {
		s.StartBlock("BindPose")
		s.StartBlock("Matrices", len(m.BindPose))
		for _, v := range m.BindPose {
			s.WriteVector(v)
		}
		s.EndBlock()
		s.EndBlock()
	}
	return s.writer.String()
}

func writeMdlObjects[T interface{ WriteMdl(*TokenStream, int) }](s *TokenStream, name string, objects []T, version int) {
	if len(objects) == 0 {
		return
	}
	if name != "" {
		s.StartBlock(name, len(objects))
	}
	for _, v := range objects {
		v.WriteMdl(s, version)
	}
	if name != "" {
		s.EndBlock()
	}
}
