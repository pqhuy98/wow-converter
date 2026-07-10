package adt

import (
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
)

// WDTEntry holds file data IDs for a map tile.
type WDTEntry struct {
	RootADT         uint32
	Obj0ADT         uint32
	Obj1ADT         uint32
	Tex0ADT         uint32
	LodADT          uint32
	MapTexture      uint32
	MapTextureN     uint32
	MinimapTexture  uint32
}

// WorldModelPlacement describes a world WMO placement from WDT.
type WorldModelPlacement struct {
	ID              uint32
	UID             uint32
	Position        [3]float32
	Rotation        [3]float32
	UpperExtents    [3]float32
	LowerExtents    [3]float32
	Flags           uint16
	DoodadSetIndex  uint16
	NameSet         uint16
	Padding         uint16
}

// WDTLoader parses .wdt map descriptor files.
type WDTLoader struct {
	Data *buffer.Buffer

	Flags                uint32
	LgtFileDataID        uint32
	OccFileDataID        uint32
	FogsFileDataID       uint32
	MpvFileDataID        uint32
	TexFileDataID        uint32
	WdlFileDataID        uint32
	Pd4FileDataID        uint32
	Tiles                []uint32
	Entries              []WDTEntry
	WorldModel           string
	WorldModelPlacement  *WorldModelPlacement
}

// NewWDTLoader creates a WDT loader.
func NewWDTLoader(data *buffer.Buffer) *WDTLoader {
	return &WDTLoader{Data: data}
}

// Load parses the WDT file.
func (w *WDTLoader) Load() {
	for w.Data.RemainingBytes() > 0 {
		chunkID := readU32(w.Data)
		chunkSize := int(readU32(w.Data))
		nextChunkPos := w.Data.Offset() + chunkSize
		if handler, ok := wdtChunkHandlers[chunkID]; ok {
			handler(w, w.Data, chunkSize)
		}
		w.Data.Seek(nextChunkPos)
	}
}

type wdtChunkHandler func(*WDTLoader, *buffer.Buffer, int)

var wdtChunkHandlers = map[uint32]wdtChunkHandler{
	0x4D504844: func(w *WDTLoader, data *buffer.Buffer, _ int) {
		w.Flags = readU32(data)
		w.LgtFileDataID = readU32(data)
		w.OccFileDataID = readU32(data)
		w.FogsFileDataID = readU32(data)
		w.MpvFileDataID = readU32(data)
		w.TexFileDataID = readU32(data)
		w.WdlFileDataID = readU32(data)
		w.Pd4FileDataID = readU32(data)
	},
	0x4D41494E: func(w *WDTLoader, data *buffer.Buffer, _ int) {
		tiles := make([]uint32, constants.Game.MapSizeSq)
		w.Tiles = tiles
		for x := 0; x < constants.Game.MapSize; x++ {
			for y := 0; y < constants.Game.MapSize; y++ {
				tiles[y*constants.Game.MapSize+x] = readU32(data)
				data.Move(4)
			}
		}
	},
	0x4D414944: func(w *WDTLoader, data *buffer.Buffer, _ int) {
		entries := make([]WDTEntry, constants.Game.MapSizeSq)
		w.Entries = entries
		for x := 0; x < constants.Game.MapSize; x++ {
			for y := 0; y < constants.Game.MapSize; y++ {
				entries[y*constants.Game.MapSize+x] = WDTEntry{
					RootADT: readU32(data), Obj0ADT: readU32(data), Obj1ADT: readU32(data),
					Tex0ADT: readU32(data), LodADT: readU32(data), MapTexture: readU32(data),
					MapTextureN: readU32(data), MinimapTexture: readU32(data),
				}
			}
		}
	},
	0x4D574D4F: func(w *WDTLoader, data *buffer.Buffer, chunkSize int) {
		w.WorldModel = stringsTrimNull(data.ReadString(chunkSize, "utf8"))
	},
	0x4D4F4446: func(w *WDTLoader, data *buffer.Buffer, _ int) {
		pos := readFloat3(data)
		rot := readFloat3(data)
		upper := readFloat3(data)
		lower := readFloat3(data)
		w.WorldModelPlacement = &WorldModelPlacement{
			ID: readU32(data), UID: readU32(data),
			Position: pos, Rotation: rot, UpperExtents: upper, LowerExtents: lower,
			Flags: uint16(readU16(data)), DoodadSetIndex: uint16(readU16(data)),
			NameSet: uint16(readU16(data)), Padding: uint16(readU16(data)),
		}
	},
}

func stringsTrimNull(s string) string {
	if i := stringsIndexByte(s, 0); i >= 0 {
		return s[:i]
	}
	return s
}

func stringsIndexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func readU32(b *buffer.Buffer) uint32 { return uint32(b.ReadUInt32LE().(int64)) }

func readI64(b *buffer.Buffer) int64 { return b.ReadInt64LE().(int64) }

func readU16(b *buffer.Buffer) uint16 { return uint16(b.ReadUInt16LE().(int64)) }

func readFloat3(b *buffer.Buffer) [3]float32 {
	raw := b.ReadFloatLE(3).([]float32)
	return [3]float32{raw[0], raw[1], raw[2]}
}
