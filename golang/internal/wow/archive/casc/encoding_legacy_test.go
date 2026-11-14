package casc

import (
	"fmt"
	"github.com/pqhuy98/wow-converter/internal/buffer"
)

func legacyParseEncodingFile(b *BaseCASC, data *buffer.Buffer, hash string) error {
	encoding, err := NewBLTEReader(data, hash, false)
	if err != nil {
		return err
	}
	magic := encoding.ReadUInt16LE().(int64)
	if uint16(magic) != encMagic {
		return fmt.Errorf("invalid encoding magic: %d", magic)
	}
	encoding.Move(1)
	hashSizeCKey := int(encoding.ReadUInt8().(int64))
	hashSizeEKey := int(encoding.ReadUInt8().(int64))
	b.EncodingTable.init(hashSizeCKey, hashSizeEKey)
	cKeyPageSize := int(encoding.ReadInt16BE().(int64)) * 1024
	encoding.Move(2)
	cKeyPageCount := int(encoding.ReadInt32BE().(int64))
	encoding.Move(4 + 1)
	specBlockSize := int(encoding.ReadInt32BE().(int64))
	encoding.Move(specBlockSize + (cKeyPageCount * (hashSizeCKey + 16)))
	pagesStart := encoding.Offset()
	for i := 0; i < cKeyPageCount; i++ {
		pageStart := pagesStart + (cKeyPageSize * i)
		encoding.Seek(pageStart)
		for encoding.Offset() < (pageStart + pagesStart) {
			keysCount := int(encoding.ReadUInt8().(int64))
			if keysCount == 0 {
				break
			}
			size := int(encoding.ReadInt40BE().(int64))
			cKey := CascKey(encoding.ReadBinaryKey(hashSizeCKey))
			eKey := CascKey(encoding.ReadBinaryKey(hashSizeEKey))
			b.EncodingTable.set(cKey, eKey, size)
			encoding.Move(hashSizeEKey * (keysCount - 1))
		}
	}
	return nil
}
