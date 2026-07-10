package adt

import (
	"fmt"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

// ADTChunk is a terrain chunk from a root ADT.
type ADTChunk struct {
	Flags                uint32
	IndexX, IndexY       uint32
	NLayers              uint32
	NDoodadRefs          uint32
	HolesHighRes         []uint8
	OfsMCLY              uint32
	OfsMCRF              uint32
	OfsMCAL              uint32
	SizeAlpha            uint32
	OfsMCSH              uint32
	SizeShadows          uint32
	AreaID               uint32
	NMapObjRefs          uint32
	HolesLowRes          uint16
	Unk1                 uint16
	LowQualityTextureMap []int16
	NoEffectDoodad       int64
	OfsMCSE              uint32
	NumMCSE              uint32
	OfsMCLQ              uint32
	SizeMCLQ             uint32
	Position             [3]float32
	OfsMCCV              uint32
	OfsMCLW              uint32
	Unk2                 uint32
	Vertices             []float32
	VertexShading        []RGBA
	Normals              [][3]int8
	BlendBatches         []BlendBatch
}

// RGBA is a vertex color.
type RGBA struct{ R, G, B, A uint8 }

// BlendBatch is a terrain blend batch entry.
type BlendBatch struct {
	MbmhIndex, IndexCount, IndexFirst, VertexCount, VertexFirst uint32
}

// LiquidInstance is a liquid layer instance.
type LiquidInstance struct {
	ChunkIndex, InstanceIndex                        int
	LiquidType, LiquidObject                         uint16
	MinHeightLevel, MaxHeightLevel                   float32
	XOffset, YOffset, Width, Height                  uint8
	Bitmap                                           []uint8
	VertexData                                       LiquidVertexData
	OffsetExistsBitmap, OffsetVertexData             uint32
}

// LiquidVertexData holds liquid vertex attributes.
type LiquidVertexData struct {
	Height []float32
	Depth  []uint8
	UV     []struct{ X, Y uint16 }
}

// LiquidChunk is liquid data for a terrain chunk.
type LiquidChunk struct {
	Attributes struct {
		Fishable, Deep uint64
	}
	Instances []LiquidInstance
}

// ADTHeader is the ADT MHDR header.
type ADTHeader struct {
	Flags                                            uint32
	OfsMCIN, OfsMTEX, OfsMMDX, OfsMMID               uint32
	OfsMWMO, OfsMWID, OfsMDDF, OfsMODF               uint32
	OfsMFBO, OfsMH20, OfsMTXF                        uint32
	Unk                                              [4]uint32
}

// TexChunkLayer is a texture layer in a tex ADT chunk.
type TexChunkLayer struct {
	TextureID, Flags, OffsetMCAL uint32
	EffectID                     int32
}

// TexChunk is texture data for a terrain chunk.
type TexChunk struct {
	Layers      []TexChunkLayer
	AlphaLayers [][]uint8
}

// TexParams holds MTXP parameters.
type TexParams struct {
	Flags, Unk3 uint32
	Height, Offset float32
}

// DoodadEntry is an M2 doodad placement.
type DoodadEntry struct {
	MmidEntry, UniqueID uint32
	Position, Rotation  [3]float32
	Scale               uint16
	Flags               uint16
}

// WorldModelEntry is a WMO placement.
type WorldModelEntry struct {
	MwidEntry, UniqueID uint32
	Position, Rotation  [3]float32
	LowerBounds, UpperBounds [3]float32
	Flags, DoodadSet, NameSet, Scale uint16
}

// ADTLoader parses ADT map tile files.
type ADTLoader struct {
	Data *buffer.Buffer
	WDT  *WDTLoader

	Version              int
	Chunks               []ADTChunk
	ChunkIndex           int
	TexChunks            []TexChunk
	Header               *ADTHeader
	LiquidChunks         []LiquidChunk
	Textures             map[int]string
	TexParams            []TexParams
	HeightTextureFileDataIDs, DiffuseTextureFileDataIDs []uint32
	M2Names              map[int]string
	M2Offsets            []uint32
	WmoNames             map[int]string
	WmoOffsets           []uint32
	Models               []DoodadEntry
	WorldModels          []WorldModelEntry
	DoodadSets           []uint16

	handlers map[uint32]adtChunkHandler
}

type adtChunkHandler func(*ADTLoader, *buffer.Buffer, int)

// NewADTLoader creates an ADT loader.
func NewADTLoader(data *buffer.Buffer) *ADTLoader {
	return &ADTLoader{Data: data}
}

// LoadRoot parses a root ADT.
func (a *ADTLoader) LoadRoot() {
	a.Chunks = make([]ADTChunk, 256)
	a.handlers = adtRootChunkHandlers
	a.load()
}

// LoadObj parses an object ADT.
func (a *ADTLoader) LoadObj() {
	a.handlers = adtObjChunkHandlers
	a.load()
}

// LoadTex parses a texture ADT.
func (a *ADTLoader) LoadTex(wdt *WDTLoader) {
	a.TexChunks = make([]TexChunk, 256)
	a.WDT = wdt
	a.handlers = adtTexChunkHandlers
	a.load()
}

func (a *ADTLoader) load() {
	for a.Data.RemainingBytes() > 0 {
		chunkID := readU32(a.Data)
		chunkSize := int(readU32(a.Data))
		nextChunkPos := a.Data.Offset() + chunkSize
		if handler, ok := a.handlers[chunkID]; ok {
			handler(a, a.Data, chunkSize)
		}
		a.Data.Seek(nextChunkPos)
	}
}

var adtRootChunkHandlers = map[uint32]adtChunkHandler{
	0x4D564552: func(a *ADTLoader, data *buffer.Buffer, _ int) {
		a.Version = int(readU32(data))
		if a.Version != 18 {
			panic(fmt.Sprintf("unexpected ADT version: %d", a.Version))
		}
	},
	0x4D434E4B: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		endOfs := data.Offset() + chunkSize
		chunk := ADTChunk{
			Flags: readU32(data), IndexX: readU32(data), IndexY: readU32(data),
			NLayers: readU32(data), NDoodadRefs: readU32(data),
			HolesHighRes: readU8Slice(data, 8),
			OfsMCLY: readU32(data), OfsMCRF: readU32(data), OfsMCAL: readU32(data),
			SizeAlpha: readU32(data), OfsMCSH: readU32(data), SizeShadows: readU32(data),
			AreaID: readU32(data), NMapObjRefs: readU32(data),
			HolesLowRes: uint16(readU16(data)), Unk1: uint16(readU16(data)),
			LowQualityTextureMap: readI16Slice(data, 8),
			NoEffectDoodad: readI64(data),
			OfsMCSE: readU32(data), NumMCSE: readU32(data),
			OfsMCLQ: readU32(data), SizeMCLQ: readU32(data),
			Position: readFloat3(data),
			OfsMCCV: readU32(data), OfsMCLW: readU32(data), Unk2: readU32(data),
		}
		a.Chunks[a.ChunkIndex] = chunk
		a.ChunkIndex++
		for data.Offset() < endOfs {
			subID := readU32(data)
			subSize := int(readU32(data))
			nextPos := data.Offset() + subSize
			if handler, ok := rootMCNKHandlers[subID]; ok {
				handler(&a.Chunks[a.ChunkIndex-1], data, subSize)
			}
			data.Seek(nextPos)
		}
	},
	0x4D48324F: parseMH2O,
	0x4D484452: func(a *ADTLoader, data *buffer.Buffer, _ int) {
		a.Header = &ADTHeader{
			Flags: readU32(data), OfsMCIN: readU32(data), OfsMTEX: readU32(data),
			OfsMMDX: readU32(data), OfsMMID: readU32(data), OfsMWMO: readU32(data),
			OfsMWID: readU32(data), OfsMDDF: readU32(data), OfsMODF: readU32(data),
			OfsMFBO: readU32(data), OfsMH20: readU32(data), OfsMTXF: readU32(data),
			Unk: [4]uint32{readU32(data), readU32(data), readU32(data), readU32(data)},
		}
	},
}

type mcnkHandler func(*ADTChunk, *buffer.Buffer, int)

var rootMCNKHandlers = map[uint32]mcnkHandler{
	0x4D435654: func(c *ADTChunk, data *buffer.Buffer, _ int) {
		c.Vertices = readFloatSlice(data, 145)
	},
	0x4D434356: func(c *ADTChunk, data *buffer.Buffer, _ int) {
		shading := make([]RGBA, 145)
		for i := 0; i < 145; i++ {
			shading[i] = RGBA{
				R: uint8(data.ReadUInt8().(int64)), G: uint8(data.ReadUInt8().(int64)),
				B: uint8(data.ReadUInt8().(int64)), A: uint8(data.ReadUInt8().(int64)),
			}
		}
		c.VertexShading = shading
	},
	0x4D434E52: func(c *ADTChunk, data *buffer.Buffer, _ int) {
		normals := make([][3]int8, 145)
		for i := 0; i < 145; i++ {
			x := int8(data.ReadInt8().(int64))
			z := int8(data.ReadInt8().(int64))
			y := int8(data.ReadInt8().(int64))
			normals[i] = [3]int8{x, y, z}
		}
		c.Normals = normals
	},
	0x4D434242: func(c *ADTChunk, data *buffer.Buffer, chunkSize int) {
		count := chunkSize / 20
		batches := make([]BlendBatch, count)
		for i := 0; i < count; i++ {
			batches[i] = BlendBatch{
				MbmhIndex: readU32(data), IndexCount: readU32(data), IndexFirst: readU32(data),
				VertexCount: readU32(data), VertexFirst: readU32(data),
			}
		}
		c.BlendBatches = batches
	},
}

func parseMH2O(a *ADTLoader, data *buffer.Buffer, _ int) {
	base := data.Offset()
	dataOffsets := make(map[int]struct{})
	chunkHeaders := make([]struct {
		offsetInstances, layerCount, offsetAttributes uint32
	}, 256)
	chunks := make([]LiquidChunk, 256)
	a.LiquidChunks = chunks

	for i := 0; i < 256; i++ {
		chunkHeaders[i] = struct {
			offsetInstances, layerCount, offsetAttributes uint32
		}{readU32(data), readU32(data), readU32(data)}
		if chunkHeaders[i].offsetAttributes > 0 {
			dataOffsets[int(chunkHeaders[i].offsetAttributes)] = struct{}{}
		}
		chunks[i].Instances = make([]LiquidInstance, chunkHeaders[i].layerCount)
	}

	var allInstances []LiquidInstance
	for i := 0; i < 256; i++ {
		header := chunkHeaders[i]
		chunk := &chunks[i]
		if header.layerCount > 0 {
			data.Seek(base + int(header.offsetInstances))
			for j := 0; j < int(header.layerCount); j++ {
				inst := LiquidInstance{
					ChunkIndex: i, InstanceIndex: j,
					LiquidType: uint16(readU16(data)), LiquidObject: uint16(readU16(data)),
					MinHeightLevel: data.ReadFloatLE().(float32), MaxHeightLevel: data.ReadFloatLE().(float32),
					XOffset: uint8(data.ReadUInt8().(int64)), YOffset: uint8(data.ReadUInt8().(int64)),
					Width: uint8(data.ReadUInt8().(int64)), Height: uint8(data.ReadUInt8().(int64)),
					OffsetExistsBitmap: readU32(data), OffsetVertexData: readU32(data),
				}
				if inst.LiquidObject <= 41 {
					inst.XOffset, inst.YOffset, inst.Width, inst.Height = 0, 0, 8, 8
				}
				if inst.OffsetExistsBitmap > 0 {
					dataOffsets[int(inst.OffsetExistsBitmap)] = struct{}{}
				}
				if inst.OffsetVertexData > 0 {
					dataOffsets[int(inst.OffsetVertexData)] = struct{}{}
				}
				chunk.Instances[j] = inst
				allInstances = append(allInstances, inst)
			}
		}
	}

	sortedOffsets := sortedKeys(dataOffsets)
	for i := 0; i < 256; i++ {
		header := chunkHeaders[i]
		chunk := &chunks[i]
		if header.offsetAttributes > 0 {
			data.Seek(base + int(header.offsetAttributes))
			chunk.Attributes.Fishable = data.ReadUInt64LE().(uint64)
			chunk.Attributes.Deep = data.ReadUInt64LE().(uint64)
		}
	}

	for idx := range allInstances {
		inst := &chunks[allInstances[idx].ChunkIndex].Instances[allInstances[idx].InstanceIndex]
		if inst.OffsetExistsBitmap > 0 {
			data.Seek(base + int(inst.OffsetExistsBitmap))
			bitmapSize := (int(inst.Width)*int(inst.Height) + 7) / 8
			inst.Bitmap = readU8Slice(data, bitmapSize)
		}
		if inst.OffsetVertexData == 0 && inst.LiquidType != 2 {
			vertexCount := (int(inst.Width) + 1) * (int(inst.Height) + 1)
			inst.VertexData.Height = make([]float32, vertexCount)
		} else if inst.OffsetVertexData > 0 {
			vertexCount := (int(inst.Width) + 1) * (int(inst.Height) + 1)
			offsetIndex := indexOf(sortedOffsets, int(inst.OffsetVertexData))
			var dataSize int
			if offsetIndex < len(sortedOffsets)-1 {
				dataSize = sortedOffsets[offsetIndex+1] - int(inst.OffsetVertexData)
			} else {
				dataSize = vertexCount * 5
			}
			data.Seek(base + int(inst.OffsetVertexData))
			bytesPerVertex := dataSize / vertexCount
			switch bytesPerVertex {
			case 5:
				inst.VertexData.Height = readFloatSlice(data, vertexCount)
				inst.VertexData.Depth = readU8Slice(data, vertexCount)
			case 8:
				inst.VertexData.Height = readFloatSlice(data, vertexCount)
				uv := make([]struct{ X, Y uint16 }, vertexCount)
				for k := 0; k < vertexCount; k++ {
					uv[k] = struct{ X, Y uint16 }{uint16(readU16(data)), uint16(readU16(data))}
				}
				inst.VertexData.UV = uv
			case 1:
				inst.VertexData.Depth = readU8Slice(data, vertexCount)
			case 9:
				inst.VertexData.Height = readFloatSlice(data, vertexCount)
				uv := make([]struct{ X, Y uint16 }, vertexCount)
				for k := 0; k < vertexCount; k++ {
					uv[k] = struct{ X, Y uint16 }{uint16(readU16(data)), uint16(readU16(data))}
				}
				inst.VertexData.UV = uv
				inst.VertexData.Depth = readU8Slice(data, vertexCount)
			}
		}
	}
}

var adtTexChunkHandlers = map[uint32]adtChunkHandler{
	0x4D564552: adtRootChunkHandlers[0x4D564552],
	0x4D544558: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		a.Textures = ReadStringBlock(data, chunkSize)
	},
	0x4D434E4B: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		endOfs := data.Offset() + chunkSize
		chunk := TexChunk{}
		a.TexChunks[a.ChunkIndex] = chunk
		idx := a.ChunkIndex
		a.ChunkIndex++
		for data.Offset() < endOfs {
			subID := readU32(data)
			subSize := int(readU32(data))
			nextPos := data.Offset() + subSize
			if handler, ok := texMCNKHandlers[subID]; ok {
				handler(&a.TexChunks[idx], data, subSize, a.WDT)
			}
			data.Seek(nextPos)
		}
	},
	0x4D545850: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		count := chunkSize / 16
		params := make([]TexParams, count)
		for i := 0; i < count; i++ {
			params[i] = TexParams{
				Flags: readU32(data), Height: data.ReadFloatLE().(float32),
				Offset: data.ReadFloatLE().(float32), Unk3: readU32(data),
			}
		}
		a.TexParams = params
	},
	0x4D484944: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		a.HeightTextureFileDataIDs = readU32Slice(data, chunkSize/4)
	},
	0x4D444944: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		a.DiffuseTextureFileDataIDs = readU32Slice(data, chunkSize/4)
	},
}

type texMCNKHandler func(*TexChunk, *buffer.Buffer, int, *WDTLoader)

var texMCNKHandlers = map[uint32]texMCNKHandler{
	0x4D434C59: func(c *TexChunk, data *buffer.Buffer, chunkSize int, _ *WDTLoader) {
		count := chunkSize / 16
		layers := make([]TexChunkLayer, count)
		for i := 0; i < count; i++ {
			layers[i] = TexChunkLayer{
				TextureID: readU32(data), Flags: readU32(data),
				OffsetMCAL: readU32(data), EffectID: int32(readI32(data)),
			}
		}
		c.Layers = layers
	},
	0x4D43414C: func(c *TexChunk, data *buffer.Buffer, _ int, wdt *WDTLoader) {
		layerCount := len(c.Layers)
		alphaLayers := make([][]uint8, layerCount)
		c.AlphaLayers = alphaLayers
		alphaLayers[0] = make([]uint8, 64*64)
		for i := range alphaLayers[0] {
			alphaLayers[0][i] = 255
		}
		ofs := 0
		for i := 1; i < layerCount; i++ {
			layer := c.Layers[i]
			if int(layer.OffsetMCAL) != ofs {
				panic("MCAL offset mis-match")
			}
			if layer.Flags&0x200 != 0 {
				alphaLayer := make([]uint8, 64*64)
				alphaLayers[i] = alphaLayer
				inOfs, outOfs := 0, 0
				for outOfs < 4096 {
					info := uint8(data.ReadUInt8().(int64))
					inOfs++
					mode := (info & 0x80) >> 7
					count := int(info & 0x7F)
					if mode != 0 {
						value := uint8(data.ReadUInt8().(int64))
						inOfs++
						for count > 0 && outOfs < 4096 {
							alphaLayer[outOfs] = value
							outOfs++
							count--
						}
					} else {
						for count > 0 && outOfs < 4096 {
							alphaLayer[outOfs] = uint8(data.ReadUInt8().(int64))
							inOfs++
							outOfs++
							count--
						}
					}
				}
				ofs += inOfs
			} else if wdt.Flags&0x4 != 0 || wdt.Flags&0x80 != 0 {
				alphaLayers[i] = readU8Slice(data, 4096)
				ofs += 4096
			} else {
				alphaLayer := make([]uint8, 64*64)
				alphaLayers[i] = alphaLayer
				rawLayer := readU8Slice(data, 2048)
				ofs += 2048
				for j := 0; j < 2048; j++ {
					alphaLayer[2*j] = (rawLayer[j] & 0x0F) * 17
					alphaLayer[2*j+1] = ((rawLayer[j] & 0xF0) >> 4) * 17
				}
			}
		}
	},
}

var adtObjChunkHandlers = map[uint32]adtChunkHandler{
	0x4D564552: adtRootChunkHandlers[0x4D564552],
	0x4D4D4458: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		a.M2Names = ReadStringBlock(data, chunkSize)
	},
	0x4D4D4944: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		a.M2Offsets = readU32Slice(data, chunkSize/4)
	},
	0x4D574D4F: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		a.WmoNames = ReadStringBlock(data, chunkSize)
	},
	0x4D574944: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		a.WmoOffsets = readU32Slice(data, chunkSize/4)
	},
	0x4D444446: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		count := chunkSize / 36
		entries := make([]DoodadEntry, count)
		for i := 0; i < count; i++ {
			entries[i] = DoodadEntry{
				MmidEntry: readU32(data), UniqueID: readU32(data),
				Position: readFloat3(data), Rotation: readFloat3(data),
				Scale: uint16(readU16(data)), Flags: uint16(readU16(data)),
			}
		}
		a.Models = entries
	},
	0x4D4F4446: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		count := chunkSize / 64
		entries := make([]WorldModelEntry, count)
		for i := 0; i < count; i++ {
			entries[i] = WorldModelEntry{
				MwidEntry: readU32(data), UniqueID: readU32(data),
				Position: readFloat3(data), Rotation: readFloat3(data),
				LowerBounds: readFloat3(data), UpperBounds: readFloat3(data),
				Flags: uint16(readU16(data)), DoodadSet: uint16(readU16(data)),
				NameSet: uint16(readU16(data)), Scale: uint16(readU16(data)),
			}
		}
		a.WorldModels = entries
	},
	0x4D574453: func(a *ADTLoader, data *buffer.Buffer, chunkSize int) {
		a.DoodadSets = readU16Slice(data, chunkSize/2)
	},
}

func readU8Slice(b *buffer.Buffer, count int) []uint8 {
	raw := b.ReadUInt8(count).([]int64)
	out := make([]uint8, count)
	for i, v := range raw {
		out[i] = uint8(v)
	}
	return out
}

func readI16Slice(b *buffer.Buffer, count int) []int16 {
	raw := b.ReadInt16LE(count).([]int64)
	out := make([]int16, count)
	for i, v := range raw {
		out[i] = int16(v)
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

func readU32Slice(b *buffer.Buffer, count int) []uint32 {
	raw := b.ReadUInt32LE(count).([]int64)
	out := make([]uint32, count)
	for i, v := range raw {
		out[i] = uint32(v)
	}
	return out
}

func readFloatSlice(b *buffer.Buffer, count int) []float32 {
	return b.ReadFloatLE(count).([]float32)
}

func readI32(b *buffer.Buffer) int32 { return int32(b.ReadInt32LE().(int64)) }

func sortedKeys(m map[int]struct{}) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func indexOf(slice []int, val int) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1
}
