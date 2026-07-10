package m2

import (
	"context"
	"fmt"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
)

const magicSkin = 0x4E494B53

// SkinSubMesh is a submesh in an M2 skin file.
type SkinSubMesh struct {
	SubmeshID, Level, VertexStart, VertexCount uint16
	TriangleStart                              uint32
	TriangleCount                              uint16
	BoneCount, BoneStart, BoneInfluences       uint16
	CenterBoneIndex                            uint16
	CenterPosition, SortCenterPosition         [3]float32
	SortRadius                                 float32
}

// SkinTextureUnit is a texture unit in an M2 skin file.
type SkinTextureUnit struct {
	Flags, Priority                                                             uint8
	ShaderID, SkinSectionIndex, Flags2                                          uint16
	ColorIndex, MaterialIndex, MaterialLayer                                    uint16
	TextureCount, TextureComboIndex                                             uint16
	TextureCoordComboIndex, TextureWeightComboIndex, TextureTransformComboIndex uint16
}

// Skin loads .skin geometry files.
type Skin struct {
	FileDataID   uint32
	FileName     string
	IsLoaded     bool
	Bones        uint32
	Indices      []uint16
	Triangles    []uint16
	Properties   []uint8
	SubMeshes    []SkinSubMesh
	TextureUnits []SkinTextureUnit

	getFile func(ctx context.Context, fileDataID uint32) ([]byte, error)
}

// NewSkin creates a skin reference by file data ID.
func NewSkin(fileDataID uint32) *Skin {
	name, _ := archivecasc.GetByID(int(fileDataID))
	if name == "" {
		name = fmt.Sprintf("unknown_%d.skin", fileDataID)
	}
	return &Skin{FileDataID: fileDataID, FileName: name}
}

// Load reads skin geometry from CASC.
func (s *Skin) Load(ctx context.Context) error {
	if s.IsLoaded {
		return nil
	}
	if s.getFile == nil {
		return fmt.Errorf("skin loader has no file source")
	}
	raw, err := s.getFile(ctx, s.FileDataID)
	if err != nil {
		return fmt.Errorf("unable to load skin fileDataID %d: %w", s.FileDataID, err)
	}
	data := buffer.From(raw)
	if readU32(data) != magicSkin {
		return fmt.Errorf("invalid skin magic for fileDataID %d", s.FileDataID)
	}
	indicesCount := int(readU32(data))
	indicesOfs := int(readU32(data))
	trianglesCount := int(readU32(data))
	trianglesOfs := int(readU32(data))
	propertiesCount := int(readU32(data))
	propertiesOfs := int(readU32(data))
	subMeshesCount := int(readU32(data))
	subMeshesOfs := int(readU32(data))
	textureUnitsCount := int(readU32(data))
	textureUnitsOfs := int(readU32(data))
	s.Bones = readU32(data)

	data.Seek(indicesOfs)
	s.Indices = readU16Slice(data, indicesCount)
	data.Seek(trianglesOfs)
	s.Triangles = readU16Slice(data, trianglesCount)
	data.Seek(propertiesOfs)
	s.Properties = readU8Slice(data, propertiesCount)
	data.Seek(subMeshesOfs)
	s.SubMeshes = make([]SkinSubMesh, subMeshesCount)
	for i := 0; i < subMeshesCount; i++ {
		sm := SkinSubMesh{
			SubmeshID: uint16(readU16(data)), Level: uint16(readU16(data)),
			VertexStart: uint16(readU16(data)), VertexCount: uint16(readU16(data)),
			TriangleStart: uint32(readU16(data)), TriangleCount: uint16(readU16(data)),
			BoneCount: uint16(readU16(data)), BoneStart: uint16(readU16(data)),
			BoneInfluences: uint16(readU16(data)), CenterBoneIndex: uint16(readU16(data)),
			CenterPosition: readFloat3(data), SortCenterPosition: readFloat3(data),
			SortRadius: data.ReadFloatLE().(float32),
		}
		sm.TriangleStart += uint32(sm.Level) << 16
		s.SubMeshes[i] = sm
	}
	data.Seek(textureUnitsOfs)
	s.TextureUnits = make([]SkinTextureUnit, textureUnitsCount)
	for i := 0; i < textureUnitsCount; i++ {
		s.TextureUnits[i] = SkinTextureUnit{
			Flags: uint8(data.ReadUInt8().(int64)), Priority: uint8(data.ReadUInt8().(int64)),
			ShaderID: uint16(readU16(data)), SkinSectionIndex: uint16(readU16(data)),
			Flags2: uint16(readU16(data)), ColorIndex: uint16(readU16(data)),
			MaterialIndex: uint16(readU16(data)), MaterialLayer: uint16(readU16(data)),
			TextureCount: uint16(readU16(data)), TextureComboIndex: uint16(readU16(data)),
			TextureCoordComboIndex: uint16(readU16(data)), TextureWeightComboIndex: uint16(readU16(data)),
			TextureTransformComboIndex: uint16(readU16(data)),
		}
	}
	s.IsLoaded = true
	return nil
}

func readU8Slice(b *buffer.Buffer, count int) []uint8 {
	raw := b.ReadUInt8(count).([]int64)
	out := make([]uint8, count)
	for i, v := range raw {
		out[i] = uint8(v)
	}
	return out
}

func readU16Slice(b *buffer.Buffer, count int) []uint16 {
	raw := b.ReadUInt16LE(count).([]int64)
	out := make([]uint16, count)
	for i, v := range raw {
		out[i] = uint16(v)
	}
	return out
}

func readFloat3(b *buffer.Buffer) [3]float32 {
	raw := b.ReadFloatLE(3).([]float32)
	return [3]float32{raw[0], raw[1], raw[2]}
}
