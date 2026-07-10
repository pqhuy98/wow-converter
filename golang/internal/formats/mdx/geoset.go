package mdx

import "strconv"

// Geoset is a mesh geoset.
type Geoset struct {
	Vertices         []float32
	Normals          []float32
	FaceTypeGroups   []uint32
	FaceGroups       []uint32
	Faces            []uint16
	VertexGroups     []uint8
	MatrixGroups     []uint32
	MatrixIndices    []uint32
	MaterialID       uint32
	SelectionGroup   uint32
	SelectionFlags   uint32
	LOD              int32
	LODName          string
	Extent           *Extent
	SequenceExtents  []*Extent
	Tangents         []float32
	Skin             []uint8
	UVSets           [][]float32
}

func NewGeoset() *Geoset {
	return &Geoset{
		LOD:     -1,
		Extent:  NewExtent(),
	}
}

func (g *Geoset) ReadMdx(stream *BinaryStream, version int) error {
	if _, err := stream.ReadUint32(); err != nil {
		return err
	}
	if err := stream.Skip(4); err != nil { // VRTX
		return err
	}
	vc, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	g.Vertices, err = stream.ReadFloat32Array(int(vc) * 3)
	if err != nil {
		return err
	}
	if err := stream.Skip(4); err != nil { // NRMS
		return err
	}
	nc, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	g.Normals, err = stream.ReadFloat32Array(int(nc) * 3)
	if err != nil {
		return err
	}
	if err := stream.Skip(4); err != nil { // PTYP
		return err
	}
	ptc, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	g.FaceTypeGroups, err = stream.ReadUint32Array(int(ptc))
	if err != nil {
		return err
	}
	if err := stream.Skip(4); err != nil { // PCNT
		return err
	}
	pgc, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	g.FaceGroups, err = stream.ReadUint32Array(int(pgc))
	if err != nil {
		return err
	}
	if err := stream.Skip(4); err != nil { // PVTX
		return err
	}
	fc, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	g.Faces, err = stream.ReadUint16Array(int(fc))
	if err != nil {
		return err
	}
	if err := stream.Skip(4); err != nil { // GNDX
		return err
	}
	vgc, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	g.VertexGroups, err = stream.ReadUint8Array(int(vgc))
	if err != nil {
		return err
	}
	if err := stream.Skip(4); err != nil { // MTGC
		return err
	}
	mgc, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	g.MatrixGroups, err = stream.ReadUint32Array(int(mgc))
	if err != nil {
		return err
	}
	if err := stream.Skip(4); err != nil { // MATS
		return err
	}
	mic, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	g.MatrixIndices, err = stream.ReadUint32Array(int(mic))
	if err != nil {
		return err
	}
	g.MaterialID, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	g.SelectionGroup, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	g.SelectionFlags, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	if version > 800 {
		g.LOD, err = stream.ReadInt32()
		if err != nil {
			return err
		}
		g.LODName, err = stream.Read(80)
		if err != nil {
			return err
		}
	}
	if err := g.Extent.ReadMdx(stream); err != nil {
		return err
	}
	sec, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	for i := uint32(0); i < sec; i++ {
		ext := NewExtent()
		if err := ext.ReadMdx(stream); err != nil {
			return err
		}
		g.SequenceExtents = append(g.SequenceExtents, ext)
	}
	if version > 800 {
		tag, err := stream.ReadBinary(4)
		if err != nil {
			return err
		}
		if tag == "TANG" {
			tc, err := stream.ReadUint32()
			if err != nil {
				return err
			}
			g.Tangents, err = stream.ReadFloat32Array(int(tc) * 4)
			if err != nil {
				return err
			}
		} else {
			stream.Seek(stream.Index() - 4)
		}
		tag, err = stream.ReadBinary(4)
		if err != nil {
			return err
		}
		if tag == "SKIN" {
			sc, err := stream.ReadUint32()
			if err != nil {
				return err
			}
			g.Skin, err = stream.ReadUint8Array(int(sc))
			if err != nil {
				return err
			}
		} else {
			stream.Seek(stream.Index() - 4)
		}
	}
	if err := stream.Skip(4); err != nil { // UVAS
		return err
	}
	uvc, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	for i := uint32(0); i < uvc; i++ {
		if err := stream.Skip(4); err != nil { // UVBS
			return err
		}
		uvn, err := stream.ReadUint32()
		if err != nil {
			return err
		}
		uv, err := stream.ReadFloat32Array(int(uvn) * 2)
		if err != nil {
			return err
		}
		g.UVSets = append(g.UVSets, uv)
	}
	return nil
}

func (g *Geoset) WriteMdx(stream *BinaryStream, version int) {
	stream.WriteUint32(uint32(g.GetByteLength(version)))
	stream.WriteBinary("VRTX")
	stream.WriteUint32(uint32(len(g.Vertices) / 3))
	stream.WriteFloat32Array(g.Vertices)
	stream.WriteBinary("NRMS")
	stream.WriteUint32(uint32(len(g.Normals) / 3))
	stream.WriteFloat32Array(g.Normals)
	stream.WriteBinary("PTYP")
	stream.WriteUint32(uint32(len(g.FaceTypeGroups)))
	stream.WriteUint32Array(g.FaceTypeGroups)
	stream.WriteBinary("PCNT")
	stream.WriteUint32(uint32(len(g.FaceGroups)))
	stream.WriteUint32Array(g.FaceGroups)
	stream.WriteBinary("PVTX")
	stream.WriteUint32(uint32(len(g.Faces)))
	stream.WriteUint16Array(g.Faces)
	stream.WriteBinary("GNDX")
	stream.WriteUint32(uint32(len(g.VertexGroups)))
	stream.WriteUint8Array(g.VertexGroups)
	stream.WriteBinary("MTGC")
	stream.WriteUint32(uint32(len(g.MatrixGroups)))
	stream.WriteUint32Array(g.MatrixGroups)
	stream.WriteBinary("MATS")
	stream.WriteUint32(uint32(len(g.MatrixIndices)))
	stream.WriteUint32Array(g.MatrixIndices)
	stream.WriteUint32(g.MaterialID)
	stream.WriteUint32(g.SelectionGroup)
	stream.WriteUint32(g.SelectionFlags)
	if version > 800 {
		stream.WriteInt32(g.LOD)
		written := stream.Write(g.LODName)
		stream.Skip(80 - written)
	}
	g.Extent.WriteMdx(stream)
	stream.WriteUint32(uint32(len(g.SequenceExtents)))
	for _, ext := range g.SequenceExtents {
		ext.WriteMdx(stream)
	}
	if version > 800 {
		if len(g.Tangents) > 0 {
			stream.WriteBinary("TANG")
			stream.WriteUint32(uint32(len(g.Tangents) / 4))
			stream.WriteFloat32Array(g.Tangents)
		}
		if len(g.Skin) > 0 {
			stream.WriteBinary("SKIN")
			stream.WriteUint32(uint32(len(g.Skin)))
			stream.WriteUint8Array(g.Skin)
		}
	}
	stream.WriteBinary("UVAS")
	stream.WriteUint32(uint32(len(g.UVSets)))
	for _, uv := range g.UVSets {
		stream.WriteBinary("UVBS")
		stream.WriteUint32(uint32(len(uv) / 2))
		stream.WriteFloat32Array(uv)
	}
}

func (g *Geoset) ReadMdl(stream *TokenStream) error {
	return stream.ReadBlockIter(func(token string) error {
		switch token {
		case "Vertices":
			n, err := stream.ReadInt()
			if err != nil {
				return err
			}
			view := make([]float32, n*3)
			_, err = stream.ReadVectorsBlock(view, 3)
			if err != nil {
				return err
			}
			g.Vertices = view
		case "Normals":
			n, err := stream.ReadInt()
			if err != nil {
				return err
			}
			view := make([]float32, n*3)
			_, err = stream.ReadVectorsBlock(view, 3)
			if err != nil {
				return err
			}
			g.Normals = view
		case "TVertices":
			n, err := stream.ReadInt()
			if err != nil {
				return err
			}
			view := make([]float32, n*2)
			_, err = stream.ReadVectorsBlock(view, 2)
			if err != nil {
				return err
			}
			g.UVSets = append(g.UVSets, view)
		case "VertexGroup":
			var groups []uint8
			err := stream.ReadBlockIter(func(vg string) error {
				n, err := strconv.Atoi(vg)
				if err != nil {
					return err
				}
				groups = append(groups, uint8(n))
				return nil
			})
			if err != nil {
				return err
			}
			g.VertexGroups = groups
		case "Tangents":
			n, err := stream.ReadInt()
			if err != nil {
				return err
			}
			view := make([]float32, n*4)
			_, err = stream.ReadVectorsBlock(view, 4)
			if err != nil {
				return err
			}
			g.Tangents = view
		case "SkinWeights":
			n, err := stream.ReadInt()
			if err != nil {
				return err
			}
			view := make([]uint8, n*8)
			_, err = stream.ReadVectorU8(view)
			if err != nil {
				return err
			}
			g.Skin = view
		case "Faces":
			g.FaceTypeGroups = []uint32{4}
			vectors, err := stream.ReadInt()
			if err != nil {
				return err
			}
			count, err := stream.ReadInt()
			if err != nil {
				return err
			}
			if _, err := stream.Read(); err != nil { // {
				return err
			}
			if _, err := stream.Read(); err != nil { // Triangles
				return err
			}
			faces := make([]uint16, count)
			_, err = stream.ReadVectorsBlockU16(faces, count/vectors)
			if err != nil {
				return err
			}
			g.Faces = faces
			g.FaceGroups = []uint32{uint32(count)}
			if _, err := stream.Read(); err != nil { // }
				return err
			}
		case "Groups":
			if _, err := stream.ReadInt(); err != nil { // matrices count
				return err
			}
			if _, err := stream.ReadInt(); err != nil { // total indices
				return err
			}
			var indices []uint32
			var groups []uint32
			err := stream.ReadBlockIter(func(_ string) error {
				size := uint32(0)
				err := stream.ReadBlockIter(func(index string) error {
					n, err := strconv.Atoi(index)
					if err != nil {
						return err
					}
					indices = append(indices, uint32(n))
					size++
					return nil
				})
				if err != nil {
					return err
				}
				groups = append(groups, size)
				return nil
			})
			if err != nil {
				return err
			}
			g.MatrixIndices = indices
			g.MatrixGroups = groups
		case "MinimumExtent":
			_, err := stream.ReadVector(g.Extent.Min)
			return err
		case "MaximumExtent":
			_, err := stream.ReadVector(g.Extent.Max)
			return err
		case "BoundsRadius":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			g.Extent.BoundsRadius = float32(v)
		case "Anim":
			ext := NewExtent()
			err := stream.ReadBlockIter(func(t string) error {
				switch t {
				case "MinimumExtent":
					_, err := stream.ReadVector(ext.Min)
					return err
				case "MaximumExtent":
					_, err := stream.ReadVector(ext.Max)
					return err
				case "BoundsRadius":
					v, err := stream.ReadFloat()
					if err != nil {
						return err
					}
					ext.BoundsRadius = float32(v)
				}
				return nil
			})
			if err != nil {
				return err
			}
			g.SequenceExtents = append(g.SequenceExtents, ext)
		case "MaterialID":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			g.MaterialID = uint32(v)
		case "SelectionGroup":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			g.SelectionGroup = uint32(v)
		case "Unselectable":
			g.SelectionFlags = 4
		case "LevelOfDetail":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			g.LOD = int32(v)
		case "Name":
			s, err := stream.Read()
			if err != nil {
				return err
			}
			g.LODName = s
		default:
			return unknownToken("Geoset", token)
		}
		return nil
	})
}

func (g *Geoset) WriteMdl(stream *TokenStream, version int) {
	stream.StartBlock("Geoset")
	stream.WriteVectorArrayBlock("Vertices", g.Vertices, 3)
	stream.WriteVectorArrayBlock("Normals", g.Normals, 3)
	for _, uv := range g.UVSets {
		stream.WriteVectorArrayBlock("TVertices", uv, 2)
	}
	if version > 800 && len(g.Tangents) > 0 {
		stream.WriteVectorArrayBlock("Tangents", g.Tangents, 4)
	}
	if len(g.VertexGroups) > 0 {
		stream.StartBlock("VertexGroup")
		for _, vg := range g.VertexGroups {
			stream.WriteLine(strconv.Itoa(int(vg)) + ",")
		}
		stream.EndBlock()
	}
	if version > 800 && len(g.Skin) > 0 {
		stream.StartBlock("SkinWeights", len(g.Skin)/8)
		for i := 0; i < len(g.Skin); i += 8 {
			stream.WriteLine(stream.floatArrayDecimals(g.Skin[i:i+8]) + ",")
		}
		stream.EndBlock()
	}
	stream.StartBlock("Faces", 1, len(g.Faces))
	stream.StartBlock("Triangles")
	stream.WriteVector(g.Faces)
	stream.EndBlock()
	stream.EndBlock()
	stream.StartBlock("Groups", len(g.MatrixGroups), len(g.MatrixIndices))
	index := 0
	for _, groupSize := range g.MatrixGroups {
		stream.WriteVectorAttrib("Matrices", g.MatrixIndices[index:index+int(groupSize)])
		index += int(groupSize)
	}
	stream.EndBlock()
	g.Extent.WriteMdl(stream)
	for _, ext := range g.SequenceExtents {
		stream.StartBlock("Anim")
		ext.WriteMdl(stream)
		stream.EndBlock()
	}
	stream.WriteNumberAttrib("MaterialID", float64(g.MaterialID))
	stream.WriteNumberAttrib("SelectionGroup", float64(g.SelectionGroup))
	if g.SelectionFlags == 4 {
		stream.WriteFlag("Unselectable")
	}
	if version > 800 {
		stream.WriteNumberAttrib("LevelOfDetail", float64(g.LOD))
		if len(g.LODName) > 0 {
			stream.WriteStringAttrib("Name", g.LODName)
		}
	}
	stream.EndBlock()
}

func (g *Geoset) GetByteLength(version int) int {
	size := 120 + len(g.Vertices)*4 + len(g.Normals)*4 + len(g.FaceTypeGroups)*4 +
		len(g.FaceGroups)*4 + len(g.Faces)*2 + len(g.VertexGroups) + len(g.MatrixGroups)*4 +
		len(g.MatrixIndices)*4 + len(g.SequenceExtents)*28
	if version > 800 {
		size += 84
		if len(g.Tangents) > 0 {
			size += 8 + len(g.Tangents)*4
		}
		if len(g.Skin) > 0 {
			size += 8 + len(g.Skin)
		}
	}
	for _, uv := range g.UVSets {
		size += 8 + len(uv)*4
	}
	return size
}
