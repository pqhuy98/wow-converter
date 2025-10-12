package casc

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
)

const (
	blteMagic      = 0x45544c42
	encTypeSalsa20 = 0x53
)

var emptyMD5 = "00000000000000000000000000000000"

type BLTEReader struct {
	blte          *ByteBuf
	buf           []byte
	blocks        []blteBlock
	blockIndex    int
	blockWriteIdx int
	partial       bool
}

type blteBlock struct {
	CompSize   int
	DecompSize int
	Hash       string
}

func NewBLTEReader(raw []byte, hash string, partial bool) (*BLTEReader, error) {
	buf := NewByteBuf(raw)
	if buf.ByteLength() < 8 {
		return nil, errors.New("[BLTE] Not enough data (< 8)")
	}
	magic := buf.ReadUInt32LE()
	if magic != blteMagic {
		return nil, errors.New("[BLTE] Invalid magic")
	}
	headerSize := int(buf.ReadInt32BE())
	origPos := buf.Offset()
	buf.Seek(0)
	var got string
	if headerSize > 0 {
		h := md5.Sum(buf.data[:headerSize])
		got = hex.EncodeToString(h[:])
	} else {
		h := md5.Sum(buf.data)
		got = hex.EncodeToString(h[:])
	}
	if got != hash {
		return nil, errors.New("[BLTE] Invalid MD5 hash")
	}
	buf.Seek(origPos)
	numBlocks := 1
	if headerSize > 0 {
		if buf.ByteLength() < 12 {
			return nil, errors.New("[BLTE] Not enough data (< 12)")
		}
		// Read frame header control bytes from current offset (origPos)
		fc0 := buf.ReadUInt8()
		fc1 := buf.ReadUInt8()
		fc2 := buf.ReadUInt8()
		fc3 := buf.ReadUInt8()
		numBlocks = int(fc1)<<16 | int(fc2)<<8 | int(fc3)
		if fc0 != 0x0F || numBlocks == 0 {
			return nil, errors.New("[BLTE] Invalid table format")
		}
		frameHeaderSize := 24*numBlocks + 12
		if headerSize != frameHeaderSize {
			return nil, errors.New("[BLTE] Invalid header size")
		}
		if buf.ByteLength() < frameHeaderSize {
			return nil, errors.New("[BLTE] Not enough data (frameHeader)")
		}
	}
	blocks := make([]blteBlock, numBlocks)
	allocSize := 0
	for i := 0; i < numBlocks; i++ {
		var b blteBlock
		if headerSize != 0 {
			b.CompSize = int(buf.ReadInt32BE())
			b.DecompSize = int(buf.ReadInt32BE())
			b.Hash = buf.ReadHexString(16)
		} else {
			b.CompSize = buf.ByteLength() - 8
			b.DecompSize = buf.ByteLength() - 9
			b.Hash = emptyMD5
		}
		allocSize += b.DecompSize
		blocks[i] = b
	}
	r := &BLTEReader{blte: buf, buf: make([]byte, allocSize), blocks: blocks, partial: partial}
	return r, nil
}

func (r *BLTEReader) processAll() {
	for r.processNext() {
	}
}

func (r *BLTEReader) processNext() bool {
	if r.blockIndex == len(r.blocks) {
		return false
	}
	old := r.blockWriteIdx
	blk := r.blocks[r.blockIndex]
	bltePos := r.blte.Offset()
	if blk.Hash != emptyMD5 {
		blockData := r.blte.ReadBuffer(blk.CompSize)
		got := blockData.CalculateMD5HexAll()
		if got != blk.Hash {
			// Strict: fail fast on integrity error
			panic("BLTEIntegrityError: expected " + blk.Hash + ", got " + got)
		}
		r.blte.Seek(bltePos) // reset
	}
	r.handleBlock(bltePos+blk.CompSize, r.blockIndex)
	r.blte.Seek(bltePos + blk.CompSize)
	r.blockIndex++
	r.blockWriteIdx = old + blk.DecompSize
	return true
}

func (r *BLTEReader) handleBlock(blockEnd int, index int) {
	flag := r.blte.ReadUInt8()
	switch flag {
	case 0x45: // encrypted
		if dec := r.decryptBlock(blockEnd, index); dec != nil {
			sub := &BLTEReader{blte: NewByteBuf(dec), buf: r.buf[r.blockWriteIdx:], blocks: []blteBlock{{CompSize: len(dec), DecompSize: len(dec), Hash: emptyMD5}}}
			sub.handleBlock(len(dec), index)
		} else if r.partial {
			r.blockWriteIdx += r.blocks[index].DecompSize
		}
	case 0x4E: // frame normal
		r.writeBuffer(blockEnd)
	case 0x5A: // compressed (zlib)
		// Decompress zlib block and write into output buffer
		comp := r.blte.data[r.blte.Offset():blockEnd]
		if dec, err := blteDecompress(comp); err == nil {
			copy(r.buf[r.blockWriteIdx:], dec)
			r.blte.Seek(blockEnd)
		} else {
			// Fallback: copy raw if decompression fails
			r.writeBuffer(blockEnd)
		}
	default:
		// Unsupported block types should fail (parity with JS throwing)
		panic("BLTE: Unsupported block type")
	}
}

func (r *BLTEReader) writeBuffer(blockEnd int) {
	copy(r.buf[r.blockWriteIdx:], r.blte.data[r.blte.Offset():blockEnd])
	r.blte.Seek(blockEnd)
}

func (r *BLTEReader) Output() []byte { r.processAll(); return r.buf }

// decryptBlock implements Salsa20-based BLTE block decryption
func (r *BLTEReader) decryptBlock(blockEnd int, index int) []byte {
	keyNameSize := int(r.blte.ReadUInt8())
	if keyNameSize == 0 || keyNameSize != 8 {
		return nil
	}
	keyNameBytes := make([]string, keyNameSize)
	for i := 0; i < keyNameSize; i++ {
		keyNameBytes[i] = r.blte.ReadHexString(1)
	}
	// reverse
	for i, j := 0, len(keyNameBytes)-1; i < j; i, j = i+1, j-1 {
		keyNameBytes[i], keyNameBytes[j] = keyNameBytes[j], keyNameBytes[i]
	}
	keyName := ""
	for _, s := range keyNameBytes {
		keyName += s
	}
	ivSize := int(r.blte.ReadUInt8())
	if (ivSize != 4 && ivSize != 8) || ivSize > 8 {
		return nil
	}
	ivShort := make([]byte, ivSize)
	copy(ivShort, r.blte.data[r.blte.Offset():r.blte.Offset()+ivSize])
	r.blte.Seek(r.blte.Offset() + ivSize)
	if r.blte.Remaining() == 0 {
		return nil
	}
	encryptType := r.blte.ReadUInt8()
	if encryptType != encTypeSalsa20 {
		return nil
	}
	for shift, i := 0, 0; i < 4; shift, i = shift+8, i+1 {
		if i < len(ivShort) {
			ivShort[i] ^= byte((index >> shift) & 0xFF)
		}
	}
	key := GetTACTKey(keyName)
	if len(key) == 0 {
		// Strict: missing key is an error unless partial mode (handled by caller)
		return nil
	}
	var nonce [8]byte
	copy(nonce[:], ivShort)
	s := newSalsa20(nonce, key, 20)
	dec := s.process(r.blte.data[r.blte.Offset():blockEnd])
	return dec
}

// blteDecompress inflates a zlib-compressed BLTE block.
func blteDecompress(comp []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(comp))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}
