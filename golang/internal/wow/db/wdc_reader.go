package db

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

// DB2FieldValue is a single field value within a DB2 row.
type DB2FieldValue any

// DB2Row is a DB2 row keyed by schema field names.
type DB2Row map[string]DB2FieldValue

var tableFormats = map[uint32]struct {
	name       string
	wdcVersion int
}{
	0x32434457: {"WDC2", 2},
	0x434C5331: {"CLS1", 2},
	0x33434457: {"WDC3", 3},
	0x34434457: {"WDC4", 4},
	0x35434457: {"WDC5", 5},
}

type sectionHeader struct {
	tactKeyHash          uint64
	fileOffset           uint32
	recordCount          uint32
	stringTableSize      uint32
	copyTableSize        uint32
	offsetMapOffset      uint32
	idListSize           uint32
	relationshipDataSize uint32
	offsetRecordsEnd     uint32
	offsetMapIDCount     uint32
	copyTableCount       uint32
}

type fieldInfo struct {
	fieldOffsetBits         uint16
	fieldSizeBits           uint16
	additionalDataSize      uint32
	fieldCompression        uint32
	fieldCompressionPacking [3]uint32
}

type section struct {
	header          sectionHeader
	isNormal        bool
	recordDataOfs   int
	recordDataSize  int
	stringBlockOfs  int
	idList          []uint32
	offsetMap       []offsetMapEntry
	relationshipMap map[uint32]uint32
}

type offsetMapEntry struct {
	offset uint32
	size   uint16
}

// WDCReader reads WDC2-WDC5 DB2 tables.
type WDCReader struct {
	FileName string
	source   FileSource

	rows               map[uint32]DB2Row
	rowOrder           []uint32
	copyTable          map[uint32]uint32
	copyTableOrder     []uint32
	stringTable        map[int]string
	schema             map[string]SchemaEntry
	schemaOrder        []string
	isInflated         bool
	isLoaded           bool
	idField            string
	idFieldIndex       int
	relationshipLookup map[uint32][]uint32
}

// NewWDCReader creates a reader for the given DB2 path.
func NewWDCReader(fileName string, source FileSource) *WDCReader {
	if source == nil {
		source = RuntimeFileSource{}
	}
	return &WDCReader{
		FileName:           fileName,
		source:             source,
		rows:               make(map[uint32]DB2Row),
		copyTable:          make(map[uint32]uint32),
		stringTable:        make(map[int]string),
		schema:             make(map[string]SchemaEntry),
		relationshipLookup: make(map[uint32][]uint32),
		idFieldIndex:       -1,
	}
}

// Size returns row count including copy table entries.
func (r *WDCReader) Size() int {
	return len(r.rows) + len(r.copyTable)
}

// GetRow returns a row by ID or nil.
func (r *WDCReader) GetRow(recordID uint32) DB2Row {
	if !r.isLoaded {
		panic("attempted to read a data table row before table was loaded")
	}
	if record, ok := r.rows[recordID]; ok {
		return record
	}
	if copyID, ok := r.copyTable[recordID]; ok {
		if copy, ok := r.rows[copyID]; ok {
			temp := make(DB2Row, len(copy))
			for k, v := range copy {
				temp[k] = v
			}
			temp["ID"] = recordID
			return temp
		}
	}
	return nil
}

// GetAllRows returns all rows, inflating copy table entries once.
func (r *WDCReader) GetAllRows() map[uint32]DB2Row {
	if !r.isLoaded {
		panic("attempted to read data table rows before table was loaded")
	}
	if !r.isInflated {
		for _, destID := range r.copyTableOrder {
			srcID := r.copyTable[destID]
			if row, ok := r.rows[srcID]; ok {
				temp := make(DB2Row, len(row))
				for k, v := range row {
					temp[k] = v
				}
				temp["ID"] = destID
				r.rows[destID] = temp
				r.rowOrder = append(r.rowOrder, destID)
			}
		}
		r.isInflated = true
	}
	return r.rows
}

// RowIDsInOrder returns row IDs in DB2 parse order, matching TS Map insertion order.
func (r *WDCReader) RowIDsInOrder() []uint32 {
	_ = r.GetAllRows()
	ids := make([]uint32, len(r.rowOrder))
	copy(ids, r.rowOrder)
	return ids
}

// GetRelationRows returns rows matching a foreign key value.
func (r *WDCReader) GetRelationRows(foreignKeyValue uint32) []DB2Row {
	if !r.isLoaded {
		panic("attempted to query relationship data before table was loaded")
	}
	ids := r.relationshipLookup[foreignKeyValue]
	if len(ids) == 0 {
		return nil
	}
	out := make([]DB2Row, 0, len(ids))
	for _, id := range ids {
		if row, ok := r.rows[id]; ok {
			out = append(out, row)
		}
	}
	return out
}

func convertDBDToSchemaType(entry *DBDField) SchemaEntry {
	if !entry.IsInline && entry.IsRelation {
		return SchemaEntry{Type: FieldTypeRelation}
	}
	if !entry.IsInline && entry.IsID {
		return SchemaEntry{Type: FieldTypeNonInlineID}
	}
	if entry.Type == DBDColumnString || entry.Type == DBDColumnLocString {
		return SchemaEntry{Type: FieldTypeString}
	}
	if entry.Type == DBDColumnFloat {
		return SchemaEntry{Type: FieldTypeFloat}
	}
	if entry.Type == DBDColumnInt {
		var ft FieldType
		switch entry.Size {
		case 8:
			if entry.IsSigned {
				ft = FieldTypeInt8
			} else {
				ft = FieldTypeUInt8
			}
		case 16:
			if entry.IsSigned {
				ft = FieldTypeInt16
			} else {
				ft = FieldTypeUInt16
			}
		case 32:
			if entry.IsSigned {
				ft = FieldTypeInt32
			} else {
				ft = FieldTypeUInt32
			}
		case 64:
			if entry.IsSigned {
				ft = FieldTypeInt64
			} else {
				ft = FieldTypeUInt64
			}
		default:
			panic(fmt.Sprintf("unsupported DBD integer size %d", entry.Size))
		}
		if entry.ArrayLength > -1 {
			return SchemaEntry{Type: ft, Count: entry.ArrayLength}
		}
		return SchemaEntry{Type: ft}
	}
	panic(fmt.Sprintf("unrecognized DBD type %s", entry.Type))
}

func (r *WDCReader) loadSchema(ctx context.Context, layoutHash string) error {
	buildID := r.source.GetBuildName()
	tableName := strings.TrimSuffix(filepath.Base(r.FileName), filepath.Ext(r.FileName))
	dbdName := tableName + ".dbd"
	log.Write("Loading table definitions %s (%s %s)...", dbdName, buildID, layoutHash)

	var rawDbd []byte
	var err error
	rawDbd, err = r.source.GetDBD(ctx, tableName)
	var structure *DBDEntry
	if err == nil {
		parser, perr := NewDBDParser(buffer.From(rawDbd))
		if perr == nil {
			structure = parser.GetStructure(buildID, layoutHash)
		}
	}
	if structure == nil {
		rawDbd, err = downloadDBD(tableName)
		if err != nil {
			return fmt.Errorf("unable to download DBD for %s: %w", tableName, err)
		}
		if err := r.source.StoreDBD(tableName, rawDbd); err != nil {
			log.Write("Failed to store DBD %s: %v", dbdName, err)
		}
		parser, perr := NewDBDParser(buffer.From(rawDbd))
		if perr != nil {
			return perr
		}
		structure = parser.GetStructure(buildID, layoutHash)
	}
	if structure == nil {
		return fmt.Errorf("no table definition available for %s", tableName)
	}
	r.buildSchemaFromDBDStructure(structure)
	return nil
}

func (r *WDCReader) buildSchemaFromDBDStructure(structure *DBDEntry) {
	for _, field := range structure.Fields {
		entry := convertDBDToSchemaType(field)
		r.schema[field.Name] = entry
		r.schemaOrder = append(r.schemaOrder, field.Name)
	}
}

// Parse loads and parses the DB2 file.
func (r *WDCReader) Parse(ctx context.Context, input []byte) error {
	log.Write("Loading DB file %s from CASC", r.FileName)
	var data *buffer.Buffer
	if input != nil {
		data = buffer.From(input)
	} else {
		raw, err := r.source.GetFileByName(ctx, r.FileName)
		if err != nil {
			return err
		}
		data = buffer.From(raw)
	}

	magic := readU32(data)
	format, ok := tableFormats[magic]
	if !ok {
		return fmt.Errorf("unsupported DB2 type: %08x", magic)
	}
	wdcVersion := format.wdcVersion
	log.Write("Processing DB file %s as %s", r.FileName, format.name)

	if wdcVersion == 5 {
		data.ReadUInt32LE()
		data.ReadUInt8(128)
	}

	recordCount := int(readU32(data))
	data.Move(4)
	recordSize := int(readU32(data))
	data.Move(4)
	data.Move(4)
	layoutBytes := readBytes(data, 4)
	layoutHash := strings.ToUpper(fmt.Sprintf("%02x%02x%02x%02x", layoutBytes[3], layoutBytes[2], layoutBytes[1], layoutBytes[0]))
	minID := readU32(data)
	maxID := readU32(data)
	data.Move(4)
	flags := uint16(readU16(data))
	idIndex := int(readU16(data))
	r.idFieldIndex = idIndex
	totalFieldCount := int(readU32(data))
	data.Move(4)
	data.Move(4)
	fieldStorageInfoSize := int(readU32(data))
	commonDataSize := int(readU32(data))
	palletDataSize := int(readU32(data))
	sectionCount := int(readU32(data))

	if err := r.loadSchema(ctx, layoutHash); err != nil {
		return err
	}

	sectionHeaders := make([]sectionHeader, sectionCount)
	for i := 0; i < sectionCount; i++ {
		if wdcVersion == 2 {
			sectionHeaders[i] = sectionHeader{
				tactKeyHash:          readU64(data),
				fileOffset:           readU32(data),
				recordCount:          readU32(data),
				stringTableSize:      readU32(data),
				copyTableSize:        readU32(data),
				offsetMapOffset:      readU32(data),
				idListSize:           readU32(data),
				relationshipDataSize: readU32(data),
			}
		} else {
			sectionHeaders[i] = sectionHeader{
				tactKeyHash:          readU64(data),
				fileOffset:           readU32(data),
				recordCount:          readU32(data),
				stringTableSize:      readU32(data),
				offsetRecordsEnd:     readU32(data),
				idListSize:           readU32(data),
				relationshipDataSize: readU32(data),
				offsetMapIDCount:     readU32(data),
				copyTableCount:       readU32(data),
			}
		}
	}

	fields := make([]struct{ size, position int16 }, totalFieldCount)
	for i := 0; i < totalFieldCount; i++ {
		fields[i].size = int16(readI16(data))
		fields[i].position = int16(readI16(data))
	}

	fieldInfoCount := fieldStorageInfoSize / (4 * 6)
	fieldInfos := make([]fieldInfo, fieldInfoCount)
	for i := 0; i < fieldInfoCount; i++ {
		fieldInfos[i] = fieldInfo{
			fieldOffsetBits:         uint16(readU16(data)),
			fieldSizeBits:           uint16(readU16(data)),
			additionalDataSize:      readU32(data),
			fieldCompression:        readU32(data),
			fieldCompressionPacking: [3]uint32{readU32(data), readU32(data), readU32(data)},
		}
	}

	prevPos := data.Offset()
	palletData := make([][]uint32, len(fieldInfos))
	for i := range fieldInfos {
		fi := &fieldInfos[i]
		if fi.fieldCompression == uint32(CompressionBitpackedIndexed) || fi.fieldCompression == uint32(CompressionBitpackedIndexedArray) {
			n := int(fi.additionalDataSize / 4)
			pallet := make([]uint32, n)
			for j := 0; j < n; j++ {
				pallet[j] = readU32(data)
			}
			palletData[i] = pallet
		}
	}
	if data.Offset() != prevPos+palletDataSize {
		return fmt.Errorf("read incorrect amount of pallet data")
	}

	prevPos = data.Offset()
	commonData := make([]map[uint32]uint32, len(fieldInfos))
	for i := range fieldInfos {
		fi := &fieldInfos[i]
		if fi.fieldCompression == uint32(CompressionCommonData) {
			m := make(map[uint32]uint32)
			for j := 0; j < int(fi.additionalDataSize/8); j++ {
				m[readU32(data)] = readU32(data)
			}
			commonData[i] = m
		}
	}
	if data.Offset() != prevPos+commonDataSize {
		return fmt.Errorf("read incorrect amount of common data")
	}

	if wdcVersion > 3 {
		for sectionIndex := 0; sectionIndex < sectionCount-1; sectionIndex++ {
			entryCount := int(readU32(data))
			data.Move(entryCount * 4)
		}
	}

	sections := make([]section, sectionCount)
	previousStringTableSize := 0
	for sectionIndex := 0; sectionIndex < sectionCount; sectionIndex++ {
		header := sectionHeaders[sectionIndex]
		isNormal := (flags & 1) == 0
		recordDataOfs := data.Offset()
		var recordsOfs uint32
		if wdcVersion == 2 {
			recordsOfs = header.offsetMapOffset
		} else {
			recordsOfs = header.offsetRecordsEnd
		}
		var recordDataSize int
		if isNormal {
			recordDataSize = recordSize * int(header.recordCount)
		} else {
			recordDataSize = int(recordsOfs) - int(header.fileOffset)
		}
		stringBlockOfs := recordDataOfs + recordDataSize

		var offsetMap []offsetMapEntry
		if wdcVersion == 2 && !isNormal {
			data.Seek(int(header.offsetMapOffset))
			offsetMapCount := int(maxID-minID) + 1
			offsetMap = make([]offsetMapEntry, offsetMapCount)
			for i := 0; i < offsetMapCount; i++ {
				offsetMap[minID+uint32(i)] = offsetMapEntry{offset: readU32(data), size: uint16(readU16(data))}
			}
		}

		if wdcVersion > 2 && isNormal {
			data.Seek(stringBlockOfs)
			for i := 0; i < int(header.stringTableSize); {
				oldPos := data.Offset()
				nul := data.IndexOf(0, data.Offset())
				if nul < 0 {
					nul = data.Offset() + data.RemainingBytes()
				}
				str := data.ReadString(nul-data.Offset(), "utf8")
				if str != "" {
					r.stringTable[i+previousStringTableSize] = str
				}
				if data.Offset() == oldPos {
					data.Seek(oldPos + 1)
				}
				i += data.Offset() - oldPos
			}
			previousStringTableSize += int(header.stringTableSize)
		}

		data.Seek(stringBlockOfs + int(header.stringTableSize))

		idListSize := int(header.idListSize / 4)
		var idList []uint32
		if idListSize > 0 {
			idList = readU32Slice(data, idListSize)
		}

		var copyTableCount int
		if wdcVersion == 2 {
			copyTableCount = int(header.copyTableSize / 8)
		} else {
			copyTableCount = int(header.copyTableCount)
		}
		for i := 0; i < copyTableCount; i++ {
			dest := uint32(readI32(data))
			src := uint32(readI32(data))
			if dest != src {
				r.copyTable[dest] = src
				r.copyTableOrder = append(r.copyTableOrder, dest)
			}
		}

		if wdcVersion > 2 {
			offsetMap = make([]offsetMapEntry, header.offsetMapIDCount)
			for i := 0; i < int(header.offsetMapIDCount); i++ {
				offsetMap[i] = offsetMapEntry{offset: readU32(data), size: uint16(readU16(data))}
			}
		}

		prevPos = data.Offset()
		var relationshipMap map[uint32]uint32
		if header.relationshipDataSize > 0 {
			relationshipEntryCount := int(readU32(data))
			data.Move(8)
			relationshipMap = make(map[uint32]uint32)
			for i := 0; i < relationshipEntryCount; i++ {
				foreignID := readU32(data)
				recordIndex := readU32(data)
				relationshipMap[recordIndex] = foreignID
			}
			if prevPos+int(header.relationshipDataSize) != data.Offset() {
				data.Seek(prevPos + int(header.relationshipDataSize))
			}
		}

		if wdcVersion > 2 {
			data.Move(int(header.offsetMapIDCount) * 4)
		}

		sections[sectionIndex] = section{
			header: header, isNormal: isNormal, recordDataOfs: recordDataOfs,
			recordDataSize: recordDataSize, stringBlockOfs: stringBlockOfs,
			idList: idList, offsetMap: offsetMap, relationshipMap: relationshipMap,
		}
	}

	castBuffer := buffer.Alloc(8, true)

	for sectionIndex := 0; sectionIndex < sectionCount; sectionIndex++ {
		sec := &sections[sectionIndex]
		header := sec.header

		if header.tactKeyHash != 0 {
			isZeroed := true
			data.Seek(sec.recordDataOfs)
			for i := 0; i < sec.recordDataSize; i++ {
				if readU8(data) != 0 {
					isZeroed = false
					break
				}
			}
			if isZeroed && wdcVersion > 2 && sec.isNormal && (header.idListSize > 0 || header.copyTableCount > 0) {
				data.Seek(sec.stringBlockOfs + int(header.stringTableSize))
				isZeroed = readU32(data) == 0
			}
			if isZeroed && wdcVersion > 2 && header.offsetMapIDCount > 0 && len(sec.offsetMap) > 0 {
				isZeroed = sec.offsetMap[0].size == 0
			}
			if isZeroed {
				log.Write("Skipping all-zero encrypted section %d in file %s", sectionIndex, r.FileName)
				continue
			}
		}

		outsideDataSize := 0
		for i := 0; i < sectionIndex; i++ {
			outsideDataSize += sections[i].recordDataSize
		}

		hasIDMap := len(sec.idList) > 0
		emptyIDMap := hasIDMap
		if emptyIDMap {
			for _, id := range sec.idList {
				if id != 0 {
					emptyIDMap = false
					break
				}
			}
		}

		for i := 0; i < int(header.recordCount); i++ {
			var recordID uint32
			if hasIDMap {
				recordID = sec.idList[i]
			}
			if hasIDMap && emptyIDMap {
				recordID = uint32(i)
			}

			var recordOfs int
			if sec.isNormal {
				recordOfs = i * recordSize
			} else if wdcVersion == 2 {
				recordOfs = int(sec.offsetMap[recordID].offset)
			} else {
				recordOfs = int(sec.offsetMap[i].offset)
			}
			absoluteRecordOffs := recordOfs - (recordCount * recordSize)

			if !sec.isNormal {
				data.Seek(recordOfs)
			} else {
				data.Seek(sec.recordDataOfs + recordOfs)
			}

			out := make(DB2Row)
			fieldIndex := 0
			for _, prop := range r.schemaOrder {
				schemaEntry := r.schema[prop]
				if schemaEntry.Type == FieldTypeRelation {
					if sec.relationshipMap != nil {
						if v, ok := sec.relationshipMap[uint32(i)]; ok {
							out[prop] = v
						} else {
							out[prop] = uint32(0)
						}
					} else {
						out[prop] = uint32(0)
					}
					continue
				}
				if schemaEntry.Type == FieldTypeNonInlineID {
					if hasIDMap {
						out[prop] = sec.idList[i]
					}
					continue
				}

				fi := &fieldInfos[fieldIndex]
				count := schemaEntry.Count
				fieldType := schemaEntry.Type
				fieldOffsetBytes := int(fi.fieldOffsetBits / 8)

				switch CompressionType(fi.fieldCompression) {
				case CompressionNone:
					switch fieldType {
					case FieldTypeString:
						if sec.isNormal {
							if count > 0 {
								arr := make([]string, count)
								out[prop] = arr
								for si := 0; si < count; si++ {
									dataPos := (int(fi.fieldOffsetBits) + (si * (int(fi.fieldSizeBits) / count))) >> 3
									ofs := readU32(data)
									stringTableIndex := outsideDataSize + absoluteRecordOffs + dataPos + int(ofs)
									if ofs == 0 || stringTableIndex == 0 {
										arr[si] = ""
									} else if s, ok := r.stringTable[stringTableIndex]; ok {
										arr[si] = s
									} else {
										return fmt.Errorf("missing stringtable entry")
									}
								}
							} else {
								dataPos := int(fi.fieldOffsetBits) >> 3
								ofs := readU32(data)
								stringTableIndex := outsideDataSize + absoluteRecordOffs + dataPos + int(ofs)
								if ofs == 0 || stringTableIndex == 0 {
									out[prop] = ""
								} else if s, ok := r.stringTable[stringTableIndex]; ok {
									out[prop] = s
								} else {
									return fmt.Errorf("missing stringtable entry")
								}
							}
						} else if count > 0 {
							arr := make([]string, count)
							out[prop] = arr
							for si := 0; si < count; si++ {
								nul := data.IndexOf(0, data.Offset())
								arr[si] = data.ReadString(nul-data.Offset(), "utf8")
								data.ReadUInt8()
							}
						} else {
							nul := data.IndexOf(0, data.Offset())
							out[prop] = data.ReadString(nul-data.Offset(), "utf8")
							data.ReadUInt8()
						}
					case FieldTypeInt8:
						out[prop] = readFieldInts(data, count, readI8AsInt64)
					case FieldTypeUInt8:
						out[prop] = readFieldInts(data, count, readU8AsInt64)
					case FieldTypeInt16:
						out[prop] = readFieldInts(data, count, readI16AsInt64)
					case FieldTypeUInt16:
						out[prop] = readFieldInts(data, count, readU16AsInt64)
					case FieldTypeInt32:
						out[prop] = readFieldInts(data, count, readI32AsInt64)
					case FieldTypeUInt32:
						out[prop] = readFieldInts(data, count, readU32AsInt64)
					case FieldTypeInt64:
						out[prop] = readFieldInts(data, count, readI64AsInt64)
					case FieldTypeUInt64:
						out[prop] = readFieldInts(data, count, readU64AsInt64)
					case FieldTypeFloat:
						out[prop] = readFieldFloats(data, count)
					default:
						return fmt.Errorf("unsupported field type: %d", fieldType)
					}
				case CompressionCommonData:
					if commonData[fieldIndex] != nil {
						if v, ok := commonData[fieldIndex][recordID]; ok {
							out[prop] = v
						} else {
							out[prop] = fi.fieldCompressionPacking[0]
						}
					} else {
						out[prop] = fi.fieldCompressionPacking[0]
					}
				case CompressionBitpacked, CompressionBitpackedSigned, CompressionBitpackedIndexed, CompressionBitpackedIndexedArray:
					data.Seek(sec.recordDataOfs + recordOfs + fieldOffsetBytes)
					var rawValue uint64
					if data.RemainingBytes() >= 8 {
						rawValue = readU64(data)
					} else {
						castBuffer.Seek(0)
						castBuffer.WriteBuffer(data, data.RemainingBytes())
						castBuffer.Seek(0)
						rawValue = readU64(castBuffer)
					}
					bitOffset := uint64(fi.fieldOffsetBits & 7)
					bitSize := uint64(1) << fi.fieldSizeBits
					bitpackedValue := (rawValue >> bitOffset) & (bitSize - 1)

					switch CompressionType(fi.fieldCompression) {
					case CompressionBitpackedIndexedArray:
						arrCount := int(fi.fieldCompressionPacking[2])
						arr := make([]uint32, arrCount)
						out[prop] = arr
						idx := int(bitpackedValue) * arrCount
						for j := 0; j < arrCount; j++ {
							arr[j] = palletData[fieldIndex][idx+j]
						}
					case CompressionBitpackedIndexed:
						palletIndex := int(bitpackedValue)
						if palletIndex < len(palletData[fieldIndex]) {
							out[prop] = palletData[fieldIndex][palletIndex]
						} else {
							return fmt.Errorf("missing pallet data entry for key %d, field %d", bitpackedValue, fieldIndex)
						}
					default:
						out[prop] = bitpackedValue
					}
					if CompressionType(fi.fieldCompression) == CompressionBitpackedSigned {
						out[prop] = signExtend(bitpackedValue, int(fi.fieldSizeBits))
					}
				default:
					return fmt.Errorf("unsupported field compression: %d", fi.fieldCompression)
				}

				if CompressionType(fi.fieldCompression) != CompressionNone {
					if count == 0 {
						val := toUint64(out[prop])
						castBuffer.Seek(0)
						if int64(val) < 0 {
							castBuffer.WriteBigInt64LE(uint64(int64(val)))
						} else {
							castBuffer.WriteBigUInt64LE(val)
						}
						castBuffer.Seek(0)
						out[prop] = reinterpretField(castBuffer, fieldType)
					} else {
						arr := out[prop].([]uint32)
						for j := 0; j < len(arr); j++ {
							castBuffer.Seek(0)
							castBuffer.WriteBigUInt64LE(uint64(arr[j]))
							castBuffer.Seek(0)
							arr[j] = uint32(toUint64(reinterpretField(castBuffer, fieldType)))
						}
					}
				}

				if fieldType == FieldTypeFloat {
					if count > 0 {
						arr := out[prop].([]float32)
						for j := range arr {
							arr[j] = float32(math.Round(float64(arr[j])*100) / 100)
						}
					} else {
						out[prop] = float32(math.Round(float64(out[prop].(float32))*100) / 100)
					}
				}

				if !hasIDMap && fieldIndex == idIndex {
					recordID = uint32(toUint64(out[prop]))
					r.idField = prop
				}
				fieldIndex++
			}

			r.rows[recordID] = out
			r.rowOrder = append(r.rowOrder, recordID)
			if sec.relationshipMap != nil {
				if foreignID, ok := sec.relationshipMap[uint32(i)]; ok {
					r.relationshipLookup[foreignID] = append(r.relationshipLookup[foreignID], recordID)
				}
			}
		}
	}

	log.Write("Parsed %s with %d rows", r.FileName, r.Size())
	r.isLoaded = true
	return nil
}

func signExtend(value uint64, bits int) int64 {
	shift := 64 - bits
	return int64(int64(value<<uint(shift)) >> uint(shift))
}

func reinterpretField(buf *buffer.Buffer, fieldType FieldType) any {
	switch fieldType {
	case FieldTypeInt8:
		return int8(readI8(buf))
	case FieldTypeUInt8:
		return readU8(buf)
	case FieldTypeInt16:
		return int16(readI16(buf))
	case FieldTypeUInt16:
		return readU16(buf)
	case FieldTypeInt32:
		return int32(readI32(buf))
	case FieldTypeUInt32:
		return readU32(buf)
	case FieldTypeInt64:
		return int64(readI64(buf))
	case FieldTypeUInt64:
		return readU64(buf)
	case FieldTypeFloat:
		return math.Float32frombits(uint32(readU32(buf)))
	default:
		panic(fmt.Sprintf("unsupported field type: %d", fieldType))
	}
}

type intReader func(*buffer.Buffer) int64

func readFieldInts(data *buffer.Buffer, count int, read intReader) any {
	if count > 0 {
		arr := make([]int64, count)
		for i := 0; i < count; i++ {
			arr[i] = read(data)
		}
		return arr
	}
	return read(data)
}

func readFieldFloats(data *buffer.Buffer, count int) any {
	if count > 0 {
		arr := make([]float32, count)
		for i := 0; i < count; i++ {
			arr[i] = readF32(data)
		}
		return arr
	}
	return readF32(data)
}

func toUint64(v any) uint64 {
	switch x := v.(type) {
	case uint64:
		return x
	case uint32:
		return uint64(x)
	case int64:
		return uint64(x)
	case int32:
		return uint64(x)
	case int:
		return uint64(x)
	default:
		return 0
	}
}

func readU8(b *buffer.Buffer) uint8   { return uint8(asInt64(b.ReadUInt8())) }
func readI8(b *buffer.Buffer) int8    { return int8(asInt64(b.ReadInt8())) }
func readU16(b *buffer.Buffer) uint16 { return uint16(asInt64(b.ReadUInt16LE())) }
func readI16(b *buffer.Buffer) int16  { return int16(asInt64(b.ReadInt16LE())) }
func readU32(b *buffer.Buffer) uint32 { return uint32(asInt64(b.ReadUInt32LE())) }
func readI32(b *buffer.Buffer) int32  { return int32(asInt64(b.ReadInt32LE())) }
func readU64(b *buffer.Buffer) uint64 {
	v := b.ReadUInt64LE()
	if arr, ok := v.([]uint64); ok && len(arr) > 0 {
		return arr[0]
	}
	if n, ok := v.(uint64); ok {
		return n
	}
	return 0
}
func readI64(b *buffer.Buffer) int64 {
	v := b.ReadInt64LE()
	if arr, ok := v.([]int64); ok && len(arr) > 0 {
		return arr[0]
	}
	if n, ok := v.(int64); ok {
		return n
	}
	if n, ok := v.(uint64); ok {
		return int64(n)
	}
	return 0
}
func readF32(b *buffer.Buffer) float32 {
	v := b.ReadFloatLE()
	if arr, ok := v.([]float32); ok && len(arr) > 0 {
		return arr[0]
	}
	if f, ok := v.(float32); ok {
		return f
	}
	return 0
}

func readI8AsInt64(b *buffer.Buffer) int64  { return int64(readI8(b)) }
func readU8AsInt64(b *buffer.Buffer) int64  { return int64(readU8(b)) }
func readI16AsInt64(b *buffer.Buffer) int64 { return int64(readI16(b)) }
func readU16AsInt64(b *buffer.Buffer) int64 { return int64(readU16(b)) }
func readI32AsInt64(b *buffer.Buffer) int64 { return int64(readI32(b)) }
func readU32AsInt64(b *buffer.Buffer) int64 { return int64(readU32(b)) }
func readI64AsInt64(b *buffer.Buffer) int64 { return readI64(b) }
func readU64AsInt64(b *buffer.Buffer) int64 { return int64(readU64(b)) }

func asInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case []int64:
		if len(x) > 0 {
			return x[0]
		}
	}
	return 0
}

func readU32Slice(b *buffer.Buffer, count int) []uint32 {
	raw := b.ReadUInt32LE(count).([]int64)
	out := make([]uint32, count)
	for i, v := range raw {
		out[i] = uint32(v)
	}
	return out
}

func readBytes(b *buffer.Buffer, n int) []byte {
	return b.ReadBuffer(buffer.ReadBufferOptions{Length: n, Wrap: false}).([]byte)
}
