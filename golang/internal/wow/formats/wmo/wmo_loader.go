package wmo

import (
	"fmt"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/adt"
)

// Fog describes WMO fog data.
type Fog struct {
	Flags       uint32
	Position    []float32
	RadiusSmall float32
	RadiusLarge float32
	Fog         struct {
		End, StartScalar float32
		Color            uint32
	}
	UnderwaterFog struct {
		End, StartScalar float32
		Color            uint32
	}
}

// Material describes a WMO material slot.
type Material struct {
	Flags       uint32
	Shader      uint32
	BlendMode   uint32
	Texture1    uint32
	Color1      uint32
	Color1b     uint32
	Texture2    uint32
	Color2      uint32
	GroupType   uint32
	Texture3    uint32
	Color3      uint32
	Flags3      uint32
	RuntimeData []uint32
}

// GroupInfo describes root-level WMO group metadata.
type GroupInfo struct {
	Flags        uint32
	BoundingBox1 []float32
	BoundingBox2 []float32
	NameIndex    int32
}

// DoodadSet describes a doodad set entry.
type DoodadSet struct {
	Name               string
	FirstInstanceIndex uint32
	DoodadCount        uint32
	Unused             uint32
}

// Doodad describes a placed doodad instance.
type Doodad struct {
	Offset   uint32
	Flags    uint8
	Position []float32
	Rotation []float32
	Scale    float32
	Color    []uint8
}

// RenderBatch describes a render batch in a WMO group.
type RenderBatch struct {
	PossibleBox1 []uint16
	PossibleBox2 []uint16
	FirstFace    uint32
	NumFaces     uint16
	FirstVertex  uint16
	LastVertex   uint16
	Flags        uint8
	MaterialID   uint8
}

// Liquid describes liquid volume data in a group.
type Liquid struct {
	VertX, VertY       uint32
	TileX, TileY       uint32
	Vertices           []LiquidVertex
	Tiles              []uint8
	Corner             []float32
	MaterialID         uint16
}

// LiquidVertex is one liquid mesh vertex.
type LiquidVertex struct {
	Data   uint32
	Height float32
}

// PortalInfo describes portal triangle metadata.
type PortalInfo struct {
	StartVertex uint16
	Count       uint16
	Plane       []float32
}

// PortalRef is a portal-to-group reference.
type PortalRef struct {
	PortalIndex uint16
	GroupIndex  uint16
	Side        int16
}

// MaterialInfo is per-face material metadata.
type MaterialInfo struct {
	Flags      uint8
	MaterialID uint8
}

// Loader parses WMO root and group files.
type Loader struct {
	data          *buffer.Buffer
	loaded        bool
	renderingOnly bool
	FileDataID    int
	FileName      string

	Version uint32

	MaterialCount uint32
	GroupCount    uint32
	PortalCount   uint32
	LightCount    uint32
	ModelCount    uint32
	DoodadCount   uint32
	SetCount      uint32
	AmbientColor  uint32
	AreaTableID   uint32
	BoundingBox1  []float32
	BoundingBox2  []float32
	Flags         uint32
	LodCount      uint16

	Groups       []*Loader
	TextureNames map[int]string
	Fogs         []Fog
	Materials    []Material

	PortalVertices [][]float32
	PortalInfo     []PortalInfo
	Mopr           []PortalRef
	GroupNames     map[int]string
	GroupInfo      []GroupInfo
	DoodadSets     []DoodadSet
	FileDataIDs    []uint32
	DoodadNames    map[int]string
	Doodads        []Doodad
	GroupIDs       []uint32

	// Group data
	Liquid         *Liquid
	VertexColours  [][]uint32
	NameOfs        uint32
	DescOfs        uint32
	OfsPortals     uint16
	NumPortals     uint16
	NumBatchesA    uint16
	NumBatchesB    uint16
	NumBatchesC    uint32
	LiquidType     uint32
	GroupID        uint32
	Indices        []uint16
	Vertices       []float32
	UVs            [][]float32
	Normals        []float32
	RenderBatches  []RenderBatch
	MaterialInfo   []MaterialInfo
}

// NewLoader creates a WMO loader.
func NewLoader(data *buffer.Buffer, fileID int, fileName string, renderingOnly bool) *Loader {
	return &Loader{data: data, FileDataID: fileID, FileName: fileName, renderingOnly: renderingOnly}
}

// Load parses all chunks in the WMO file.
func (l *Loader) Load() error {
	if l.loaded {
		return nil
	}
	data := l.data
	for data.RemainingBytes() > 0 {
		chunkID := uint32(data.ReadUInt32LE().(int64))
		chunkSize := int(data.ReadUInt32LE().(int64))
		nextChunkPos := data.Offset() + chunkSize

		if handler, ok := wmoChunkHandlers[chunkID]; ok {
			if !l.renderingOnly || !wmoOptionalChunks[chunkID] {
				handler(l, data, chunkSize)
			}
		}
		data.Seek(nextChunkPos)
	}
	l.loaded = true
	l.data = nil
	return nil
}

var wmoOptionalChunks = map[uint32]bool{
	0x4D4C4951: true, // MLIQ
	0x4D464F47: true, // MFOG
	0x4D4F5056: true, // MOPV
	0x4D4F5052: true, // MOPR
	0x4D4F5054: true, // MOPT
	0x4D4F4356: true, // MOCV
	0x4D44414C: true, // MDAL
}

type chunkHandler func(l *Loader, data *buffer.Buffer, chunkSize int)

var wmoChunkHandlers = map[uint32]chunkHandler{
	0x4D564552: handleMVER,
	0x4D4F4844: handleMOHD,
	0x4D4F5458: handleMOTX,
	0x4D464F47: handleMFOG,
	0x4D4F4D54: handleMOMT,
	0x4D4F5056: handleMOPV,
	0x4D4F5054: handleMOPT,
	0x4D4F5052: handleMOPR,
	0x4D4F474E: handleMOGN,
	0x4D4F4749: handleMOGI,
	0x4D4F4453: handleMODS,
	0x4D4F4449: handleMODI,
	0x4D4F444E: handleMODN,
	0x4D4F4444: handleMODD,
	0x47464944: handleGFID,
	0x4D4C4951: handleMLIQ,
	0x4D4F4356: handleMOCV,
	0x4D44414C: handleMDAL,
	0x4D4F4750: handleMOGP,
	0x4D4F5649: handleMOVI,
	0x4D4F5654: handleMOVT,
	0x4D4F5456: handleMOTV,
	0x4D4F4E52: handleMONR,
	0x4D4F4241: handleMOBA,
	0x4D4F5059: handleMOPY,
}

func handleMVER(l *Loader, data *buffer.Buffer, _ int) {
	l.Version = uint32(data.ReadUInt32LE().(int64))
	if l.Version != 17 {
		panic(fmt.Sprintf("unsupported WMO version: %d", l.Version))
	}
}

func handleMOHD(l *Loader, data *buffer.Buffer, _ int) {
	l.MaterialCount = uint32(data.ReadUInt32LE().(int64))
	l.GroupCount = uint32(data.ReadUInt32LE().(int64))
	l.PortalCount = uint32(data.ReadUInt32LE().(int64))
	l.LightCount = uint32(data.ReadUInt32LE().(int64))
	l.ModelCount = uint32(data.ReadUInt32LE().(int64))
	l.DoodadCount = uint32(data.ReadUInt32LE().(int64))
	l.SetCount = uint32(data.ReadUInt32LE().(int64))
	l.AmbientColor = uint32(data.ReadUInt32LE().(int64))
	l.AreaTableID = uint32(data.ReadUInt32LE().(int64))
	l.BoundingBox1 = readFloat32Slice(data, 3)
	l.BoundingBox2 = readFloat32Slice(data, 3)
	l.Flags = uint32(data.ReadUInt16LE().(int64))
	l.LodCount = uint16(data.ReadUInt16LE().(int64))
	l.Groups = make([]*Loader, l.GroupCount)
}

func handleMOTX(l *Loader, data *buffer.Buffer, chunkSize int) {
	l.TextureNames = adt.ReadStringBlock(data, chunkSize)
}

func handleMFOG(l *Loader, data *buffer.Buffer, chunkSize int) {
	count := chunkSize / 48
	fogs := make([]Fog, count)
	for i := 0; i < count; i++ {
		fogs[i].Flags = uint32(data.ReadUInt32LE().(int64))
		fogs[i].Position = readFloat32Slice(data, 3)
		fogs[i].RadiusSmall = data.ReadFloatLE().(float32)
		fogs[i].RadiusLarge = data.ReadFloatLE().(float32)
		fogs[i].Fog.End = data.ReadFloatLE().(float32)
		fogs[i].Fog.StartScalar = data.ReadFloatLE().(float32)
		fogs[i].Fog.Color = uint32(data.ReadUInt32LE().(int64))
		fogs[i].UnderwaterFog.End = data.ReadFloatLE().(float32)
		fogs[i].UnderwaterFog.StartScalar = data.ReadFloatLE().(float32)
		fogs[i].UnderwaterFog.Color = uint32(data.ReadUInt32LE().(int64))
	}
	l.Fogs = fogs
}

func handleMOMT(l *Loader, data *buffer.Buffer, chunkSize int) {
	count := chunkSize / 64
	materials := make([]Material, count)
	for i := 0; i < count; i++ {
		materials[i] = Material{
			Flags:       uint32(data.ReadUInt32LE().(int64)),
			Shader:      uint32(data.ReadUInt32LE().(int64)),
			BlendMode:   uint32(data.ReadUInt32LE().(int64)),
			Texture1:    uint32(data.ReadUInt32LE().(int64)),
			Color1:      uint32(data.ReadUInt32LE().(int64)),
			Color1b:     uint32(data.ReadUInt32LE().(int64)),
			Texture2:    uint32(data.ReadUInt32LE().(int64)),
			Color2:      uint32(data.ReadUInt32LE().(int64)),
			GroupType:   uint32(data.ReadUInt32LE().(int64)),
			Texture3:    uint32(data.ReadUInt32LE().(int64)),
			Color3:      uint32(data.ReadUInt32LE().(int64)),
			Flags3:      uint32(data.ReadUInt32LE().(int64)),
			RuntimeData: readUInt32Slice(data, 4),
		}
	}
	l.Materials = materials
}

func handleMOPV(l *Loader, data *buffer.Buffer, chunkSize int) {
	vertexCount := chunkSize / (3 * 4)
	verts := make([][]float32, vertexCount)
	for i := 0; i < vertexCount; i++ {
		verts[i] = readFloat32Slice(data, 3)
	}
	l.PortalVertices = verts
}

func handleMOPT(l *Loader, data *buffer.Buffer, _ int) {
	l.PortalInfo = make([]PortalInfo, l.PortalCount)
	for i := 0; i < int(l.PortalCount); i++ {
		l.PortalInfo[i] = PortalInfo{
			StartVertex: uint16(data.ReadUInt16LE().(int64)),
			Count:       uint16(data.ReadUInt16LE().(int64)),
			Plane:       readFloat32Slice(data, 4),
		}
	}
}

func handleMOPR(l *Loader, data *buffer.Buffer, chunkSize int) {
	entryCount := chunkSize / 8
	refs := make([]PortalRef, entryCount)
	for i := 0; i < entryCount; i++ {
		refs[i] = PortalRef{
			PortalIndex: uint16(data.ReadUInt16LE().(int64)),
			GroupIndex:  uint16(data.ReadUInt16LE().(int64)),
			Side:        int16(data.ReadInt16LE().(int64)),
		}
		data.Move(4)
	}
	l.Mopr = refs
}

func handleMOGN(l *Loader, data *buffer.Buffer, chunkSize int) {
	l.GroupNames = adt.ReadStringBlock(data, chunkSize)
}

func handleMOGI(l *Loader, data *buffer.Buffer, chunkSize int) {
	count := chunkSize / 32
	info := make([]GroupInfo, count)
	for i := 0; i < count; i++ {
		info[i] = GroupInfo{
			Flags:        uint32(data.ReadUInt32LE().(int64)),
			BoundingBox1: readFloat32Slice(data, 3),
			BoundingBox2: readFloat32Slice(data, 3),
			NameIndex:    int32(data.ReadInt32LE().(int64)),
		}
	}
	l.GroupInfo = info
}

func handleMODS(l *Loader, data *buffer.Buffer, chunkSize int) {
	count := chunkSize / 32
	sets := make([]DoodadSet, count)
	for i := 0; i < count; i++ {
		sets[i] = DoodadSet{
			Name:               strings.ReplaceAll(data.ReadString(20, ""), "\x00", ""),
			FirstInstanceIndex: uint32(data.ReadUInt32LE().(int64)),
			DoodadCount:        uint32(data.ReadUInt32LE().(int64)),
			Unused:             uint32(data.ReadUInt32LE().(int64)),
		}
	}
	l.DoodadSets = sets
}

func handleMODI(l *Loader, data *buffer.Buffer, chunkSize int) {
	l.FileDataIDs = readUInt32Slice(data, chunkSize/4)
}

func handleMODN(l *Loader, data *buffer.Buffer, chunkSize int) {
	l.DoodadNames = adt.ReadStringBlock(data, chunkSize)
	for ofs, file := range l.DoodadNames {
		l.DoodadNames[ofs] = strings.ToLower(strings.ReplaceAll(file, ".mdx", ".m2"))
	}
}

func handleMODD(l *Loader, data *buffer.Buffer, chunkSize int) {
	count := chunkSize / 40
	doodads := make([]Doodad, count)
	for i := 0; i < count; i++ {
		doodads[i] = Doodad{
			Offset:   uint32(data.ReadUInt24LE().(int64)),
			Flags:    uint8(data.ReadUInt8().(int64)),
			Position: readFloat32Slice(data, 3),
			Rotation: readFloat32Slice(data, 4),
			Scale:    data.ReadFloatLE().(float32),
			Color:    readUInt8Slice(data, 4),
		}
	}
	l.Doodads = doodads
}

func handleGFID(l *Loader, data *buffer.Buffer, chunkSize int) {
	l.GroupIDs = readUInt32Slice(data, chunkSize/4)
}

func handleMLIQ(l *Loader, data *buffer.Buffer, _ int) {
	liquidVertsX := uint32(data.ReadUInt32LE().(int64))
	liquidVertsY := uint32(data.ReadUInt32LE().(int64))
	liquidTilesX := uint32(data.ReadUInt32LE().(int64))
	liquidTilesY := uint32(data.ReadUInt32LE().(int64))
	corner := readFloat32Slice(data, 3)
	liquidMaterialID := uint16(data.ReadUInt16LE().(int64))

	vertCount := int(liquidVertsX * liquidVertsY)
	vertices := make([]LiquidVertex, vertCount)
	for i := 0; i < vertCount; i++ {
		vertices[i] = LiquidVertex{
			Data:   uint32(data.ReadUInt32LE().(int64)),
			Height: data.ReadFloatLE().(float32),
		}
	}
	tileCount := int(liquidTilesX * liquidTilesY)
	tiles := readUInt8Slice(data, tileCount)
	l.Liquid = &Liquid{
		VertX: liquidVertsX, VertY: liquidVertsY,
		TileX: liquidTilesX, TileY: liquidTilesY,
		Vertices: vertices, Tiles: tiles, Corner: corner, MaterialID: liquidMaterialID,
	}
}

func handleMOCV(l *Loader, data *buffer.Buffer, chunkSize int) {
	vals := readUInt32Slice(data, chunkSize/4)
	l.VertexColours = append(l.VertexColours, vals)
}

func handleMDAL(l *Loader, data *buffer.Buffer, _ int) {
	l.AmbientColor = uint32(data.ReadUInt32LE().(int64))
}

func handleMOGP(l *Loader, data *buffer.Buffer, chunkSize int) {
	endOfs := data.Offset() + chunkSize
	l.NameOfs = uint32(data.ReadUInt32LE().(int64))
	l.DescOfs = uint32(data.ReadUInt32LE().(int64))
	l.Flags = uint32(data.ReadUInt32LE().(int64))
	l.BoundingBox1 = readFloat32Slice(data, 3)
	l.BoundingBox2 = readFloat32Slice(data, 3)
	l.OfsPortals = uint16(data.ReadUInt16LE().(int64))
	l.NumPortals = uint16(data.ReadUInt16LE().(int64))
	l.NumBatchesA = uint16(data.ReadUInt16LE().(int64))
	l.NumBatchesB = uint16(data.ReadUInt16LE().(int64))
	l.NumBatchesC = uint32(data.ReadUInt32LE().(int64))
	data.Move(4)
	l.LiquidType = uint32(data.ReadUInt32LE().(int64))
	l.GroupID = uint32(data.ReadUInt32LE().(int64))
	data.Move(8)

	for data.Offset() < endOfs {
		chunkID := uint32(data.ReadUInt32LE().(int64))
		subChunkSize := int(data.ReadUInt32LE().(int64))
		nextChunkPos := data.Offset() + subChunkSize
		dispatchWMOChunk(l, data, chunkID, subChunkSize)
		data.Seek(nextChunkPos)
	}
}

func dispatchWMOChunk(l *Loader, data *buffer.Buffer, chunkID uint32, chunkSize int) {
	switch chunkID {
	case 0x4D564552:
		handleMVER(l, data, chunkSize)
	case 0x4D4F4844:
		handleMOHD(l, data, chunkSize)
	case 0x4D4F5458:
		handleMOTX(l, data, chunkSize)
	case 0x4D464F47:
		handleMFOG(l, data, chunkSize)
	case 0x4D4F4D54:
		handleMOMT(l, data, chunkSize)
	case 0x4D4F5056:
		handleMOPV(l, data, chunkSize)
	case 0x4D4F5054:
		handleMOPT(l, data, chunkSize)
	case 0x4D4F5052:
		handleMOPR(l, data, chunkSize)
	case 0x4D4F474E:
		handleMOGN(l, data, chunkSize)
	case 0x4D4F4749:
		handleMOGI(l, data, chunkSize)
	case 0x4D4F4453:
		handleMODS(l, data, chunkSize)
	case 0x4D4F4449:
		handleMODI(l, data, chunkSize)
	case 0x4D4F444E:
		handleMODN(l, data, chunkSize)
	case 0x4D4F4444:
		handleMODD(l, data, chunkSize)
	case 0x47464944:
		handleGFID(l, data, chunkSize)
	case 0x4D4C4951:
		handleMLIQ(l, data, chunkSize)
	case 0x4D4F4356:
		handleMOCV(l, data, chunkSize)
	case 0x4D44414C:
		handleMDAL(l, data, chunkSize)
	case 0x4D4F4750:
		handleMOGP(l, data, chunkSize)
	case 0x4D4F5649:
		handleMOVI(l, data, chunkSize)
	case 0x4D4F5654:
		handleMOVT(l, data, chunkSize)
	case 0x4D4F5456:
		handleMOTV(l, data, chunkSize)
	case 0x4D4F4E52:
		handleMONR(l, data, chunkSize)
	case 0x4D4F4241:
		handleMOBA(l, data, chunkSize)
	case 0x4D4F5059:
		handleMOPY(l, data, chunkSize)
	}
}

func handleMOVI(l *Loader, data *buffer.Buffer, chunkSize int) {
	l.Indices = readUInt16Slice(data, chunkSize/2)
}

func handleMOVT(l *Loader, data *buffer.Buffer, chunkSize int) {
	count := chunkSize / 4
	vertices := make([]float32, count)
	for i := 0; i < count; i += 3 {
		vertices[i] = data.ReadFloatLE().(float32)
		vertices[i+2] = data.ReadFloatLE().(float32) * -1
		vertices[i+1] = data.ReadFloatLE().(float32)
	}
	l.Vertices = vertices
}

func handleMOTV(l *Loader, data *buffer.Buffer, chunkSize int) {
	count := chunkSize / 4
	uvs := make([]float32, count)
	for i := 0; i < count; i += 2 {
		uvs[i] = data.ReadFloatLE().(float32)
		uvs[i+1] = (data.ReadFloatLE().(float32) - 1) * -1
	}
	l.UVs = append(l.UVs, uvs)
}

func handleMONR(l *Loader, data *buffer.Buffer, chunkSize int) {
	count := chunkSize / 4
	normals := make([]float32, count)
	for i := 0; i < count; i += 3 {
		normals[i] = data.ReadFloatLE().(float32)
		normals[i+2] = data.ReadFloatLE().(float32) * -1
		normals[i+1] = data.ReadFloatLE().(float32)
	}
	l.Normals = normals
}

func handleMOBA(l *Loader, data *buffer.Buffer, chunkSize int) {
	count := chunkSize / 24
	batches := make([]RenderBatch, count)
	for i := 0; i < count; i++ {
		batches[i] = RenderBatch{
			PossibleBox1: readUInt16Slice(data, 3),
			PossibleBox2: readUInt16Slice(data, 3),
			FirstFace:    uint32(data.ReadUInt32LE().(int64)),
			NumFaces:     uint16(data.ReadUInt16LE().(int64)),
			FirstVertex:  uint16(data.ReadUInt16LE().(int64)),
			LastVertex:   uint16(data.ReadUInt16LE().(int64)),
			Flags:        uint8(data.ReadUInt8().(int64)),
			MaterialID:   uint8(data.ReadUInt8().(int64)),
		}
	}
	l.RenderBatches = batches
}

func handleMOPY(l *Loader, data *buffer.Buffer, chunkSize int) {
	count := chunkSize / 2
	info := make([]MaterialInfo, count)
	for i := 0; i < count; i++ {
		info[i] = MaterialInfo{
			Flags:      uint8(data.ReadUInt8().(int64)),
			MaterialID: uint8(data.ReadUInt8().(int64)),
		}
	}
	l.MaterialInfo = info
}

func readFloat32Slice(data *buffer.Buffer, n int) []float32 {
	raw := data.ReadFloatLE(n).([]float32)
	return raw
}

func readUInt32Slice(data *buffer.Buffer, n int) []uint32 {
	raw := data.ReadUInt32LE(n).([]int64)
	out := make([]uint32, len(raw))
	for i, v := range raw {
		out[i] = uint32(v)
	}
	return out
}

func readUInt16Slice(data *buffer.Buffer, n int) []uint16 {
	raw := data.ReadUInt16LE(n).([]int64)
	out := make([]uint16, len(raw))
	for i, v := range raw {
		out[i] = uint16(v)
	}
	return out
}

func readUInt8Slice(data *buffer.Buffer, n int) []uint8 {
	raw := data.ReadUInt8(n).([]int64)
	out := make([]uint8, len(raw))
	for i, v := range raw {
		out[i] = uint8(v)
	}
	return out
}
