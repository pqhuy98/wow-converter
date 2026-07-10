package mdx

import (
	"fmt"
	"strings"
)

// Model is a Warcraft 3 model supporting MDX and MDL formats.
type Model struct {
	Version                  int
	Name                     string
	AnimationFile            string
	Extent                   *Extent
	BlendTime                uint32
	Sequences                []*Sequence
	GlobalSequences          []uint32
	Materials                []*Material
	Textures                 []*Texture
	TextureAnimations        []*TextureAnimation
	Geosets                  []*Geoset
	GeosetAnimations         []*GeosetAnimation
	Bones                    []*Bone
	Lights                   []*Light
	Helpers                  []*Helper
	Attachments              []*Attachment
	PivotPoints              [][]float32
	ParticleEmitters         []*ParticleEmitter
	ParticleEmitters2        []*ParticleEmitter2
	ParticleEmittersPopcorn  []*ParticleEmitterPopcorn
	RibbonEmitters           []*RibbonEmitter
	Cameras                  []*Camera
	EventObjects             []*EventObject
	CollisionShapes          []*CollisionShape
	FaceEffects              []*FaceEffect
	BindPose                 [][]float32
	UnknownChunks            []*UnknownChunk
}

// NewModel creates an empty model with defaults.
func NewModel() *Model {
	return &Model{
		Version: 800,
		Extent:  NewExtent(),
	}
}

// ConvertMdlToMdx parses MDL text and returns MDX binary data.
func ConvertMdlToMdx(mdlText string) ([]byte, error) {
	model := NewModel()
	if err := model.LoadMdl(mdlText); err != nil {
		return nil, err
	}
	return model.SaveMdx(), nil
}

// LoadMdl loads the model from MDL text.
func (m *Model) LoadMdl(buffer string) error {
	stream := NewTokenStream(buffer)
	for {
		token := stream.ReadToken()
		if token == "" {
			break
		}
		var err error
		switch token {
		case "Version":
			err = m.loadVersionBlock(stream)
		case "Model":
			err = m.loadModelBlock(stream)
		case "Sequences":
			err = m.loadNumberedObjectBlockSequences(stream)
		case "GlobalSequences":
			err = m.loadGlobalSequenceBlock(stream)
		case "Textures":
			err = m.loadNumberedObjectBlockTextures(stream)
		case "Materials":
			err = m.loadNumberedObjectBlockMaterials(stream)
		case "TextureAnims":
			err = m.loadNumberedObjectBlockTextureAnims(stream)
		case "Geoset":
			g := NewGeoset()
			err = g.ReadMdl(stream)
			if err == nil {
				m.Geosets = append(m.Geosets, g)
			}
		case "GeosetAnim":
			ga := NewGeosetAnimation()
			err = ga.ReadMdl(stream)
			if err == nil {
				m.GeosetAnimations = append(m.GeosetAnimations, ga)
			}
		case "Bone":
			b := NewBone()
			err = b.ReadMdl(stream)
			if err == nil {
				m.Bones = append(m.Bones, b)
			}
		case "Light":
			l := NewLight()
			err = l.ReadMdl(stream)
			if err == nil {
				m.Lights = append(m.Lights, l)
			}
		case "Helper":
			h := NewHelper()
			err = h.ReadMdl(stream)
			if err == nil {
				m.Helpers = append(m.Helpers, h)
			}
		case "Attachment":
			a := NewAttachment()
			err = a.ReadMdl(stream)
			if err == nil {
				m.Attachments = append(m.Attachments, a)
			}
		case "PivotPoints":
			err = m.loadPivotPointBlock(stream)
		case "ParticleEmitter":
			p := NewParticleEmitter()
			err = p.ReadMdl(stream)
			if err == nil {
				m.ParticleEmitters = append(m.ParticleEmitters, p)
			}
		case "ParticleEmitter2":
			p := NewParticleEmitter2()
			err = p.ReadMdl(stream)
			if err == nil {
				m.ParticleEmitters2 = append(m.ParticleEmitters2, p)
			}
		case "ParticleEmitterPopcorn":
			p := NewParticleEmitterPopcorn()
			err = p.ReadMdl(stream)
			if err == nil {
				m.ParticleEmittersPopcorn = append(m.ParticleEmittersPopcorn, p)
			}
		case "RibbonEmitter":
			r := NewRibbonEmitter()
			err = r.ReadMdl(stream)
			if err == nil {
				m.RibbonEmitters = append(m.RibbonEmitters, r)
			}
		case "Camera":
			c := NewCamera()
			err = c.ReadMdl(stream)
			if err == nil {
				m.Cameras = append(m.Cameras, c)
			}
		case "EventObject":
			e := NewEventObject()
			err = e.ReadMdl(stream)
			if err == nil {
				m.EventObjects = append(m.EventObjects, e)
			}
		case "CollisionShape":
			c := NewCollisionShape()
			err = c.ReadMdl(stream)
			if err == nil {
				m.CollisionShapes = append(m.CollisionShapes, c)
			}
		case "FaceFX":
			f := NewFaceEffect()
			err = f.ReadMdl(stream)
			if err == nil {
				m.FaceEffects = append(m.FaceEffects, f)
			}
		case "BindPose":
			err = m.loadBindPoseBlock(stream)
		default:
			return fmt.Errorf("Unsupported block: %s", token)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Model) loadVersionBlock(stream *TokenStream) error {
	return stream.ReadBlockIter(func(token string) error {
		if token == "FormatVersion" {
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			m.Version = v
			return nil
		}
		return fmt.Errorf("Unknown token in Version: %q", token)
	})
}

func (m *Model) loadModelBlock(stream *TokenStream) error {
	name, err := stream.Read()
	if err != nil {
		return err
	}
	m.Name = name
	return stream.ReadBlockIter(func(token string) error {
		if strings.HasPrefix(token, "Num") {
			_, err := stream.Read()
			return err
		}
		switch token {
		case "BlendTime":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			m.BlendTime = uint32(v)
		case "MinimumExtent":
			_, err := stream.ReadVector(m.Extent.Min)
			return err
		case "MaximumExtent":
			_, err := stream.ReadVector(m.Extent.Max)
			return err
		case "BoundsRadius":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			m.Extent.BoundsRadius = float32(v)
		case "AnimationFile":
			s, err := stream.Read()
			if err != nil {
				return err
			}
			m.AnimationFile = s
		default:
			return fmt.Errorf("Unknown token in Model: %q", token)
		}
		return nil
	})
}

func (m *Model) loadNumberedObjectBlockSequences(stream *TokenStream) error {
	if _, err := stream.Read(); err != nil {
		return err
	}
	return stream.ReadBlockIter(func(token string) error {
		if token == "Anim" {
			s := NewSequence()
			if err := s.ReadMdl(stream); err != nil {
				return err
			}
			m.Sequences = append(m.Sequences, s)
			return nil
		}
		return fmt.Errorf("Unknown token in Anim: %q", token)
	})
}

func (m *Model) loadNumberedObjectBlockTextures(stream *TokenStream) error {
	if _, err := stream.Read(); err != nil {
		return err
	}
	return stream.ReadBlockIter(func(token string) error {
		if token == "Bitmap" {
			t := NewTexture()
			if err := t.ReadMdl(stream); err != nil {
				return err
			}
			m.Textures = append(m.Textures, t)
			return nil
		}
		return fmt.Errorf("Unknown token in Bitmap: %q", token)
	})
}

func (m *Model) loadNumberedObjectBlockMaterials(stream *TokenStream) error {
	if _, err := stream.Read(); err != nil {
		return err
	}
	return stream.ReadBlockIter(func(token string) error {
		if token == "Material" {
			mat := NewMaterial()
			if err := mat.ReadMdl(stream); err != nil {
				return err
			}
			m.Materials = append(m.Materials, mat)
			return nil
		}
		return fmt.Errorf("Unknown token in Material: %q", token)
	})
}

func (m *Model) loadNumberedObjectBlockTextureAnims(stream *TokenStream) error {
	if _, err := stream.Read(); err != nil {
		return err
	}
	return stream.ReadBlockIter(func(token string) error {
		if token == "TVertexAnim" || token == "TVertexAnim " {
			ta := NewTextureAnimation()
			if err := ta.ReadMdl(stream); err != nil {
				return err
			}
			m.TextureAnimations = append(m.TextureAnimations, ta)
			return nil
		}
		return fmt.Errorf("Unknown token in TVortexAnim: %q", token)
	})
}

func (m *Model) loadGlobalSequenceBlock(stream *TokenStream) error {
	if _, err := stream.Read(); err != nil {
		return err
	}
	return stream.ReadBlockIter(func(token string) error {
		if token == "Duration" {
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			m.GlobalSequences = append(m.GlobalSequences, uint32(v))
			return nil
		}
		return fmt.Errorf("Unknown token in GlobalSequences: %q", token)
	})
}

func (m *Model) loadPivotPointBlock(stream *TokenStream) error {
	count, err := stream.ReadInt()
	if err != nil {
		return err
	}
	if _, err := stream.Read(); err != nil { // {
		return err
	}
	for i := 0; i < count; i++ {
		view := make([]float32, 3)
		if _, err := stream.ReadVector(view); err != nil {
			return err
		}
		m.PivotPoints = append(m.PivotPoints, view)
	}
	_, err = stream.Read() // }
	return err
}

func (m *Model) loadBindPoseBlock(stream *TokenStream) error {
	return stream.ReadBlockIter(func(token string) error {
		if token == "Matrices" {
			matrices, err := stream.ReadInt()
			if err != nil {
				return err
			}
			if _, err := stream.Read(); err != nil { // {
				return err
			}
			for i := 0; i < matrices; i++ {
				view := make([]float32, 12)
				if _, err := stream.ReadVector(view); err != nil {
					return err
				}
				m.BindPose = append(m.BindPose, view)
			}
			_, err = stream.Read() // }
			return err
		}
		return fmt.Errorf("Unknown token in BindPose: %q", token)
	})
}

// SaveMdx serializes the model to MDX binary.
func (m *Model) SaveMdx() []byte {
	stream := NewBinaryStreamWithCapacity(m.GetByteLength())
	stream.WriteBinary("MDLX")
	m.saveVersionChunk(stream)
	m.saveModelChunk(stream)
	m.saveStaticObjectChunkSequences(stream)
	m.saveGlobalSequenceChunk(stream)
	m.saveDynamicObjectChunkMaterials(stream)
	m.saveStaticObjectChunkTextures(stream)
	m.saveDynamicObjectChunkTextureAnims(stream)
	m.saveDynamicObjectChunkGeosets(stream)
	m.saveDynamicObjectChunkGeosetAnims(stream)
	m.saveDynamicObjectChunkBones(stream)
	m.saveDynamicObjectChunkLights(stream)
	m.saveDynamicObjectChunkHelpers(stream)
	m.saveDynamicObjectChunkAttachments(stream)
	m.savePivotPointChunk(stream)
	m.saveDynamicObjectChunkParticleEmitters(stream)
	m.saveDynamicObjectChunkParticleEmitters2(stream)
	if m.Version > 800 {
		m.saveDynamicObjectChunkParticleEmittersPopcorn(stream)
	}
	m.saveDynamicObjectChunkRibbonEmitters(stream)
	m.saveDynamicObjectChunkCameras(stream)
	m.saveDynamicObjectChunkEventObjects(stream)
	m.saveDynamicObjectChunkCollisionShapes(stream)
	if m.Version > 800 {
		m.saveStaticObjectChunkFaceEffects(stream)
		m.saveBindPoseChunk(stream)
	}
	for _, chunk := range m.UnknownChunks {
		chunk.WriteMdx(stream)
	}
	return stream.Bytes()
}

func (m *Model) saveVersionChunk(stream *BinaryStream) {
	stream.WriteBinary("VERS")
	stream.WriteUint32(4)
	stream.WriteUint32(uint32(m.Version))
}

func (m *Model) saveModelChunk(stream *BinaryStream) {
	stream.WriteBinary("MODL")
	stream.WriteUint32(372)
	written := stream.Write(m.Name)
	stream.Skip(80 - written)
	written = stream.Write(m.AnimationFile)
	stream.Skip(260 - written)
	m.Extent.WriteMdx(stream)
	stream.WriteUint32(m.BlendTime)
}

func (m *Model) saveStaticObjectChunkSequences(stream *BinaryStream) {
	if len(m.Sequences) == 0 {
		return
	}
	stream.WriteBinary("SEQS")
	stream.WriteUint32(uint32(len(m.Sequences) * 132))
	for _, obj := range m.Sequences {
		obj.WriteMdx(stream)
	}
}

func (m *Model) saveStaticObjectChunkTextures(stream *BinaryStream) {
	if len(m.Textures) == 0 {
		return
	}
	stream.WriteBinary("TEXS")
	stream.WriteUint32(uint32(len(m.Textures) * 268))
	for _, obj := range m.Textures {
		obj.WriteMdx(stream)
	}
}

func (m *Model) saveStaticObjectChunkFaceEffects(stream *BinaryStream) {
	if len(m.FaceEffects) == 0 {
		return
	}
	stream.WriteBinary("FAFX")
	stream.WriteUint32(uint32(len(m.FaceEffects) * 340))
	for _, obj := range m.FaceEffects {
		obj.WriteMdx(stream)
	}
}

func (m *Model) saveGlobalSequenceChunk(stream *BinaryStream) {
	if len(m.GlobalSequences) == 0 {
		return
	}
	stream.WriteBinary("GLBS")
	stream.WriteUint32(uint32(len(m.GlobalSequences) * 4))
	for _, gs := range m.GlobalSequences {
		stream.WriteUint32(gs)
	}
}

func (m *Model) saveDynamicObjectChunkMaterials(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "MTLS", m.materialsByteLength(), func(s *BinaryStream) {
		for _, obj := range m.Materials {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkTextureAnims(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "TXAN", m.sumByteLengthTextureAnims(), func(s *BinaryStream) {
		for _, obj := range m.TextureAnimations {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkGeosets(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "GEOS", m.sumByteLengthGeosets(), func(s *BinaryStream) {
		for _, obj := range m.Geosets {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkGeosetAnims(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "GEOA", m.sumByteLengthGeosetAnims(), func(s *BinaryStream) {
		for _, obj := range m.GeosetAnimations {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkBones(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "BONE", m.sumByteLengthBones(), func(s *BinaryStream) {
		for _, obj := range m.Bones {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkLights(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "LITE", m.sumByteLengthLights(), func(s *BinaryStream) {
		for _, obj := range m.Lights {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkHelpers(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "HELP", m.sumByteLengthHelpers(), func(s *BinaryStream) {
		for _, obj := range m.Helpers {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkAttachments(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "ATCH", m.sumByteLengthAttachments(), func(s *BinaryStream) {
		for _, obj := range m.Attachments {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkParticleEmitters(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "PREM", m.sumByteLengthParticleEmitters(), func(s *BinaryStream) {
		for _, obj := range m.ParticleEmitters {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkParticleEmitters2(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "PRE2", m.sumByteLengthParticleEmitters2(), func(s *BinaryStream) {
		for _, obj := range m.ParticleEmitters2 {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkParticleEmittersPopcorn(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "CORN", m.sumByteLengthParticleEmittersPopcorn(), func(s *BinaryStream) {
		for _, obj := range m.ParticleEmittersPopcorn {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkRibbonEmitters(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "RIBB", m.sumByteLengthRibbonEmitters(), func(s *BinaryStream) {
		for _, obj := range m.RibbonEmitters {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkCameras(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "CAMS", m.sumByteLengthCameras(), func(s *BinaryStream) {
		for _, obj := range m.Cameras {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkEventObjects(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "EVTS", m.sumByteLengthEventObjects(), func(s *BinaryStream) {
		for _, obj := range m.EventObjects {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunkCollisionShapes(stream *BinaryStream) {
	m.saveDynamicObjectChunk(stream, "CLID", m.sumByteLengthCollisionShapes(), func(s *BinaryStream) {
		for _, obj := range m.CollisionShapes {
			obj.WriteMdx(s, m.Version)
		}
	})
}

func (m *Model) saveDynamicObjectChunk(stream *BinaryStream, name string, size int, write func(*BinaryStream)) {
	if size == 0 {
		return
	}
	stream.WriteBinary(name)
	stream.WriteUint32(uint32(size))
	write(stream)
}

func (m *Model) savePivotPointChunk(stream *BinaryStream) {
	if len(m.PivotPoints) == 0 {
		return
	}
	stream.WriteBinary("PIVT")
	stream.WriteUint32(uint32(len(m.PivotPoints) * 12))
	for _, pp := range m.PivotPoints {
		stream.WriteFloat32Array(pp)
	}
}

func (m *Model) saveBindPoseChunk(stream *BinaryStream) {
	if len(m.BindPose) == 0 {
		return
	}
	stream.WriteBinary("BPOS")
	stream.WriteUint32(uint32(4 + len(m.BindPose)*48))
	stream.WriteUint32(uint32(len(m.BindPose)))
	for _, matrix := range m.BindPose {
		stream.WriteFloat32Array(matrix)
	}
}

// GetByteLength returns the MDX byte size.
func (m *Model) GetByteLength() int {
	size := 396
	size += m.getStaticObjectsChunkByteLength(len(m.Sequences), 132)
	size += m.getStaticObjectsChunkByteLength(len(m.GlobalSequences), 4)
	size += m.getDynamicObjectsChunkByteLength(m.materialsByteLength())
	size += m.getStaticObjectsChunkByteLength(len(m.Textures), 268)
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthTextureAnims())
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthGeosets())
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthGeosetAnims())
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthBones())
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthLights())
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthHelpers())
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthAttachments())
	size += m.getStaticObjectsChunkByteLength(len(m.PivotPoints), 12)
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthParticleEmitters())
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthParticleEmitters2())
	if m.Version > 800 {
		size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthParticleEmittersPopcorn())
	}
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthRibbonEmitters())
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthCameras())
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthEventObjects())
	size += m.getDynamicObjectsChunkByteLength(m.sumByteLengthCollisionShapes())
	if m.Version > 800 {
		size += m.getStaticObjectsChunkByteLength(len(m.FaceEffects), 340)
		size += m.getBindPoseChunkByteLength()
	}
	for _, chunk := range m.UnknownChunks {
		size += chunk.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) getStaticObjectsChunkByteLength(count, objSize int) int {
	if count > 0 {
		return 8 + count*objSize
	}
	return 0
}

func (m *Model) getDynamicObjectsChunkByteLength(objectsSize int) int {
	if objectsSize > 0 {
		return 8 + objectsSize
	}
	return 0
}

func (m *Model) getBindPoseChunkByteLength() int {
	if len(m.BindPose) > 0 {
		return 12 + len(m.BindPose)*48
	}
	return 0
}

func (m *Model) materialsByteLength() int {
	size := 0
	for _, obj := range m.Materials {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthTextureAnims() int {
	size := 0
	for _, obj := range m.TextureAnimations {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthGeosets() int {
	size := 0
	for _, obj := range m.Geosets {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthGeosetAnims() int {
	size := 0
	for _, obj := range m.GeosetAnimations {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthBones() int {
	size := 0
	for _, obj := range m.Bones {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthLights() int {
	size := 0
	for _, obj := range m.Lights {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthHelpers() int {
	size := 0
	for _, obj := range m.Helpers {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthAttachments() int {
	size := 0
	for _, obj := range m.Attachments {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthParticleEmitters() int {
	size := 0
	for _, obj := range m.ParticleEmitters {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthParticleEmitters2() int {
	size := 0
	for _, obj := range m.ParticleEmitters2 {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthParticleEmittersPopcorn() int {
	size := 0
	for _, obj := range m.ParticleEmittersPopcorn {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthRibbonEmitters() int {
	size := 0
	for _, obj := range m.RibbonEmitters {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthCameras() int {
	size := 0
	for _, obj := range m.Cameras {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthEventObjects() int {
	size := 0
	for _, obj := range m.EventObjects {
		size += obj.GetByteLength(m.Version)
	}
	return size
}

func (m *Model) sumByteLengthCollisionShapes() int {
	size := 0
	for _, obj := range m.CollisionShapes {
		size += obj.GetByteLength(m.Version)
	}
	return size
}
