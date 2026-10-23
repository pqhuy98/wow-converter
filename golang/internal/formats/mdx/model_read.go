package mdx

import "fmt"

// LoadMdx reads a complete model, retaining unrecognized chunks for round trips.
func (m *Model) LoadMdx(raw []byte) error {
	if len(raw) < 4 || string(raw[:4]) != "MDLX" {
		return fmt.Errorf("invalid MDX signature")
	}
	*m = *NewModel()
	s := NewBinaryStream(raw[4:])
	for s.Index() < len(raw)-4 {
		if len(raw)-4-s.Index() < 8 {
			return fmt.Errorf("truncated MDX chunk header")
		}
		tag, _ := s.Read(4)
		size, err := s.ReadUint32()
		if err != nil {
			return err
		}
		chunk, err := s.Substream(int(size))
		if err != nil {
			return fmt.Errorf("%s: %w", tag, err)
		}
		switch tag {
		case "VERS":
			v, e := chunk.ReadUint32()
			err = e
			m.Version = int(v)
		case "MODL":
			m.Name, err = chunk.Read(80)
			if err == nil {
				m.AnimationFile, err = chunk.Read(260)
			}
			if err == nil {
				err = m.Extent.ReadMdx(chunk)
			}
			if err == nil {
				m.BlendTime, err = chunk.ReadUint32()
			}
		case "SEQS":
			err = readMdxObjects(chunk, &m.Sequences, NewSequence, func(v *Sequence) error { return v.ReadMdx(chunk) })
		case "GLBS":
			m.GlobalSequences, err = chunk.ReadUint32Array(int(size) / 4)
		case "TEXS":
			err = readMdxObjects(chunk, &m.Textures, NewTexture, func(v *Texture) error { return v.ReadMdx(chunk) })
		case "MTLS":
			err = readMdxObjects(chunk, &m.Materials, NewMaterial, func(v *Material) error { return v.ReadMdx(chunk, m.Version) })
		case "TXAN":
			err = readMdxObjects(chunk, &m.TextureAnimations, NewTextureAnimation, func(v *TextureAnimation) error { return v.ReadMdx(chunk) })
		case "GEOS":
			err = readMdxObjects(chunk, &m.Geosets, NewGeoset, func(v *Geoset) error { return v.ReadMdx(chunk, m.Version) })
		case "GEOA":
			err = readMdxObjects(chunk, &m.GeosetAnimations, NewGeosetAnimation, func(v *GeosetAnimation) error { return v.ReadMdx(chunk) })
		case "BONE":
			err = readMdxObjects(chunk, &m.Bones, NewBone, func(v *Bone) error { return v.ReadMdx(chunk, m.Version) })
		case "HELP":
			err = readMdxObjects(chunk, &m.Helpers, NewHelper, func(v *Helper) error { return v.ReadMdx(chunk, m.Version) })
		case "ATCH":
			err = readMdxObjects(chunk, &m.Attachments, NewAttachment, func(v *Attachment) error { return v.ReadMdx(chunk, m.Version) })
		case "LITE":
			err = readMdxObjects(chunk, &m.Lights, NewLight, func(v *Light) error { return v.ReadMdx(chunk, m.Version) })
		case "PREM":
			err = readMdxObjects(chunk, &m.ParticleEmitters, NewParticleEmitter, func(v *ParticleEmitter) error { return v.ReadMdx(chunk, m.Version) })
		case "PRE2":
			err = readMdxObjects(chunk, &m.ParticleEmitters2, NewParticleEmitter2, func(v *ParticleEmitter2) error { return v.ReadMdx(chunk, m.Version) })
		case "CORN":
			err = readMdxObjects(chunk, &m.ParticleEmittersPopcorn, NewParticleEmitterPopcorn, func(v *ParticleEmitterPopcorn) error { return v.ReadMdx(chunk, m.Version) })
		case "RIBB":
			err = readMdxObjects(chunk, &m.RibbonEmitters, NewRibbonEmitter, func(v *RibbonEmitter) error { return v.ReadMdx(chunk, m.Version) })
		case "CAMS":
			err = readMdxObjects(chunk, &m.Cameras, NewCamera, func(v *Camera) error { return v.ReadMdx(chunk, m.Version) })
		case "EVTS":
			err = readMdxObjects(chunk, &m.EventObjects, NewEventObject, func(v *EventObject) error { return v.ReadMdx(chunk, m.Version) })
		case "CLID":
			err = readMdxObjects(chunk, &m.CollisionShapes, NewCollisionShape, func(v *CollisionShape) error { return v.ReadMdx(chunk, m.Version) })
		case "FAFX":
			err = readMdxObjects(chunk, &m.FaceEffects, NewFaceEffect, func(v *FaceEffect) error { return v.ReadMdx(chunk) })
		case "PIVT":
			for chunk.Index() < int(size) && err == nil {
				var v []float32
				v, err = chunk.ReadFloat32Array(3)
				m.PivotPoints = append(m.PivotPoints, v)
			}
		case "BPOS":
			var n uint32
			n, err = chunk.ReadUint32()
			if err == nil && uint64(n)*48 > uint64(size)-4 {
				return fmt.Errorf("invalid bind pose count")
			}
			for i := uint32(0); i < n && err == nil; i++ {
				var v []float32
				v, err = chunk.ReadFloat32Array(12)
				m.BindPose = append(m.BindPose, v)
			}
		default:
			m.UnknownChunks = append(m.UnknownChunks, &UnknownChunk{Tag: tag, Chunk: append([]byte(nil), chunk.Bytes()...)})
		}
		if err != nil {
			return fmt.Errorf("%s: %w", tag, err)
		}
	}
	return nil
}

func readMdxObjects[T any](s *BinaryStream, out *[]*T, create func() *T, read func(*T) error) error {
	for s.Index() < len(s.Bytes()) {
		start := s.Index()
		v := create()
		if err := read(v); err != nil {
			return err
		}
		if s.Index() <= start {
			return fmt.Errorf("MDX object reader made no progress")
		}
		*out = append(*out, v)
	}
	return nil
}
