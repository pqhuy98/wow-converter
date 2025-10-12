package casc

import (
	"errors"
)

const (
	encMagic  = 0x4E45     // 'EN'
	rootMagic = 0x4D465354 // 'TSFM'
)

type rootType struct {
	contentFlags uint32
	localeFlags  uint32
}

// parseEncodingFile fills encodingSizes and encodingKeys maps using an encoding BLTE and its hash.
func parseEncodingFile(raw []byte, hash string, encodingSizes map[string]int64, encodingKeys map[string]string) error {
	blte, err := NewBLTEReader(raw, hash, false)
	if err != nil {
		return err
	}
	buf := NewByteBuf(blte.Output())
	if uint16(buf.ReadUInt16LE()) != encMagic {
		return errors.New("invalid encoding magic")
	}
	buf.Move(1) // version
	hashSizeCKey := int(buf.ReadUInt8())
	hashSizeEKey := int(buf.ReadUInt8())
	cKeyPageSize := int(buf.ReadInt16BE()) * 1024
	buf.Move(2) // eKeyPageSize
	cKeyPageCount := int(buf.ReadInt32BE())
	buf.Move(4 + 1) // eKeyPageCount + unk11
	specBlockSize := int(buf.ReadInt32BE())
	// Skip spec block + per-page digests (cKeyPageCount * (hashSizeCKey + 16))
	buf.Move(specBlockSize + cKeyPageCount*(hashSizeCKey+16))
	pagesStart := buf.Offset()
	for i := 0; i < cKeyPageCount; i++ {
		pageStart := pagesStart + (cKeyPageSize * i)
		buf.Seek(pageStart)
		for {
			keysCount := int(buf.ReadUInt8())
			if keysCount == 0 {
				break
			}
			size := buf.ReadInt40BE()
			cKey := buf.ReadHexString(hashSizeCKey)
			encodingSizes[cKey] = size
			eKey := buf.ReadHexString(hashSizeEKey)
			encodingKeys[cKey] = eKey
			// skip remaining eKeys in the record
			buf.Move(hashSizeEKey * (keysCount - 1))
		}
	}
	return nil
}

// parseRootFile fills rootTypes and rootEntries from a root BLTE and its hash.
// Returns number of root entries parsed.
func parseRootFile(raw []byte, hash string, rootTypes *[]rootType, rootEntries map[uint32]map[int]string) (int, error) {
	blte, err := NewBLTEReader(raw, hash, false)
	if err != nil {
		return 0, err
	}
	buf := NewByteBuf(blte.Output())
	magic := buf.ReadUInt32LE()
	if magic == rootMagic {
		headerSize := buf.ReadUInt32LE()
		version := buf.ReadUInt32LE()
		if headerSize != 0x18 {
			version = 0
		} else if version != 1 && version != 2 {
			return 0, errors.New("unknown root version")
		}
		var totalFileCount uint32
		var namedFileCount uint32
		if version == 0 {
			totalFileCount = headerSize
			namedFileCount = version
			headerSize = 12
		} else {
			totalFileCount = buf.ReadUInt32LE()
			namedFileCount = buf.ReadUInt32LE()
		}
		_ = totalFileCount
		_ = namedFileCount
		buf.Seek(int(headerSize))
		allowNameless := totalFileCount != namedFileCount
		for buf.Remaining() > 0 {
			numRecords := int(buf.ReadUInt32LE())
			var contentFlags uint32
			var localeFlags uint32
			if version == 0 || version == 1 {
				contentFlags = buf.ReadUInt32LE()
				localeFlags = buf.ReadUInt32LE()
			} else { // version 2
				localeFlags = buf.ReadUInt32LE()
				c1 := buf.ReadUInt32LE()
				c2 := buf.ReadUInt32LE()
				c3 := uint32(buf.ReadUInt8())
				contentFlags = c1 | c2 | (c3 << 17)
			}
			fileDataIDs := make([]uint32, numRecords)
			var fileDataID uint32
			for i := 0; i < numRecords; i++ {
				nextID := fileDataID + uint32(buf.ReadInt32LE())
				fileDataIDs[i] = nextID
				fileDataID = nextID + 1
			}
			// content keys
			for i := 0; i < numRecords; i++ {
				fdid := fileDataIDs[i]
				entry := rootEntries[fdid]
				if entry == nil {
					entry = make(map[int]string)
					rootEntries[fdid] = entry
				}
				entry[len(*rootTypes)] = buf.ReadHexString(16)
			}
			if !(allowNameless && (contentFlags&ContentNoNameHash) != 0) {
				buf.Move(8 * numRecords)
			}
			*rootTypes = append(*rootTypes, rootType{contentFlags: contentFlags, localeFlags: localeFlags})
		}
	} else { // Classic
		buf.Seek(0)
		for buf.Remaining() > 0 {
			numRecords := int(buf.ReadUInt32LE())
			contentFlags := buf.ReadUInt32LE()
			localeFlags := buf.ReadUInt32LE()
			fileDataIDs := make([]uint32, numRecords)
			var fileDataID uint32
			for i := 0; i < numRecords; i++ {
				nextID := fileDataID + uint32(buf.ReadInt32LE())
				fileDataIDs[i] = nextID
				fileDataID = nextID + 1
			}
			for i := 0; i < numRecords; i++ {
				key := buf.ReadHexString(16)
				buf.Move(8) // hash
				fdid := fileDataIDs[i]
				entry := rootEntries[fdid]
				if entry == nil {
					entry = make(map[int]string)
					rootEntries[fdid] = entry
				}
				entry[len(*rootTypes)] = key
			}
			*rootTypes = append(*rootTypes, rootType{contentFlags: contentFlags, localeFlags: localeFlags})
		}
	}
	return len(rootEntries), nil
}
