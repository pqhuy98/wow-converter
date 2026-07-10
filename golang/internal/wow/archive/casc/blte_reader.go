package casc

import (
	"fmt"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

const (
	blteMagic        = 0x45544C42
	encTypeSalsa20   = 0x53
	emptyHash        = "00000000000000000000000000000000"
)

// EncryptionError indicates a missing decryption key.
type EncryptionError struct {
	Key string
}

func (e *EncryptionError) Error() string {
	return fmt.Sprintf("[BLTE] Missing decryption key %s", e.Key)
}

// BLTEIntegrityError indicates invalid block data hash.
type BLTEIntegrityError struct {
	Expected string
	Actual   string
}

func (e *BLTEIntegrityError) Error() string {
	return fmt.Sprintf("[BLTE] Invalid block data hash. Expected %s, got %s!", e.Expected, e.Actual)
}

// BLTEBlock describes a BLTE frame block.
type BLTEBlock struct {
	CompSize   int
	DecompSize int
	Hash       string
}

// BLTEReader decodes BLTE-compressed CASC data lazily.
type BLTEReader struct {
	*buffer.Buffer
	blte           *buffer.Buffer
	blockIndex     int
	blockWriteIndex int
	partialDecrypt bool
	blocks         []BLTEBlock
}

// CheckBLTE reports whether data is a BLTE file.
func CheckBLTE(data *buffer.Buffer) bool {
	if data.ByteLength() < 4 {
		return false
	}
	magic := data.ReadUInt32LE().(int64)
	data.Seek(0)
	return uint32(magic) == blteMagic
}

// NewBLTEReader creates a BLTE reader for the given encoded data and MD5 hash.
func NewBLTEReader(buf *buffer.Buffer, hash string, partialDecrypt bool) (*BLTEReader, error) {
	r := &BLTEReader{
		Buffer:         buffer.Alloc(0, false),
		blte:           buf,
		partialDecrypt: partialDecrypt,
	}
	size := buf.ByteLength()
	if size < 8 {
		return nil, fmt.Errorf("[BLTE] Not enough data (< 8)")
	}
	magic := buf.ReadUInt32LE().(int64)
	if uint32(magic) != blteMagic {
		return nil, fmt.Errorf("[BLTE] Invalid magic: %d", magic)
	}
	headerSize := int(buf.ReadInt32BE().(int64))
	origPos := buf.Offset()
	buf.Seek(0)
	var hashCheck string
	if headerSize > 0 {
		hashCheck = buf.ReadBuffer(buffer.ReadBufferOptions{Length: headerSize, Wrap: true}).(*buffer.Buffer).CalculateHash("md5", "hex")
	} else {
		hashCheck = buf.CalculateHash("md5", "hex")
	}
	if hashCheck != hash {
		return nil, fmt.Errorf("[BLTE] Invalid MD5 hash, expected %s got %s", hash, hashCheck)
	}
	buf.Seek(origPos)
	numBlocks := 1
	if headerSize > 0 {
		if size < 12 {
			return nil, fmt.Errorf("[BLTE] Not enough data (< 12)")
		}
		fc := buf.ReadUInt8(4).([]int64)
		numBlocks = (int(fc[1]) << 16) | (int(fc[2]) << 8) | int(fc[3])
		if fc[0] != 0x0F || numBlocks == 0 {
			return nil, fmt.Errorf("[BLTE] Invalid table format.")
		}
		frameHeaderSize := 24*numBlocks + 12
		if headerSize != frameHeaderSize {
			return nil, fmt.Errorf("[BLTE] Invalid header size.")
		}
		if size < frameHeaderSize {
			return nil, fmt.Errorf("[BLTE] Not enough data (frameHeader).")
		}
	}
	allocSize := 0
	r.blocks = make([]BLTEBlock, numBlocks)
	for i := 0; i < numBlocks; i++ {
		var block BLTEBlock
		if headerSize != 0 {
			block = BLTEBlock{
				CompSize:   int(buf.ReadInt32BE().(int64)),
				DecompSize: int(buf.ReadInt32BE().(int64)),
				Hash:       buf.ReadHexString(16),
			}
		} else {
			block = BLTEBlock{
				CompSize:   size - 8,
				DecompSize: size - 9,
				Hash:       emptyHash,
			}
		}
		allocSize += block.DecompSize
		r.blocks[i] = block
	}
	r.SetBuf(make([]byte, allocSize))
	return r, nil
}

// ProcessAllBlocks processes all BLTE blocks.
func (r *BLTEReader) ProcessAllBlocks() error {
	for r.blockIndex < len(r.blocks) {
		if err := r.processBlock(); err != nil {
			return err
		}
	}
	return nil
}

func (r *BLTEReader) processBlock() error {
	if r.blockIndex == len(r.blocks) {
		return nil
	}
	oldPos := r.Offset()
	r.Seek(r.blockWriteIndex)
	block := r.blocks[r.blockIndex]
	bltePos := r.blte.Offset()
	if block.Hash != emptyHash {
		blockData := r.blte.ReadBuffer(buffer.ReadBufferOptions{Length: block.CompSize, Wrap: true}).(*buffer.Buffer)
		blockHash := blockData.CalculateHash("md5", "hex")
		r.blte.Seek(bltePos)
		if blockHash != block.Hash {
			return &BLTEIntegrityError{Expected: block.Hash, Actual: blockHash}
		}
	}
	if err := r.handleBlock(r.blte, bltePos+block.CompSize, r.blockIndex); err != nil {
		return err
	}
	r.blte.Seek(bltePos + block.CompSize)
	r.blockIndex++
	r.blockWriteIndex = r.Offset()
	r.Seek(oldPos)
	return nil
}

func (r *BLTEReader) handleBlock(block *buffer.Buffer, blockEnd int, index int) error {
	flag := block.ReadUInt8().(int64)
	switch byte(flag) {
	case 0x45:
		decrypted, err := r.decryptBlock(block, blockEnd, index)
		if err != nil {
			if encErr, ok := err.(*EncryptionError); ok {
				if r.partialDecrypt {
					r.SetOffset(r.Offset() + r.blocks[index].DecompSize)
					return nil
				}
				return encErr
			}
			return err
		}
		return r.handleBlock(decrypted, decrypted.ByteLength(), index)
	case 0x46:
		return fmt.Errorf("[BLTE] No frame decoder implemented!")
	case 0x4E:
		r.writeBufferBLTE(block, blockEnd)
		return nil
	case 0x5A:
		r.decompressBlock(block, blockEnd, index)
		return nil
	default:
		return fmt.Errorf("Unknown block: %d", flag)
	}
}

func (r *BLTEReader) decompressBlock(data *buffer.Buffer, blockEnd int, index int) {
	decomp := data.ReadBuffer(buffer.ReadBufferOptions{
		Length:  blockEnd - data.Offset(),
		Wrap:    true,
		Inflate: true,
	}).(*buffer.Buffer)
	expectedSize := r.blocks[index].DecompSize
	if decomp.ByteLength() > expectedSize {
		r.SetCapacity(r.ByteLength()+(decomp.ByteLength()-expectedSize), false)
	}
	r.writeBufferBLTE(decomp, decomp.ByteLength())
}

func (r *BLTEReader) decryptBlock(data *buffer.Buffer, blockEnd int, index int) (*buffer.Buffer, error) {
	keyNameSize := int(data.ReadUInt8().(int64))
	if keyNameSize == 0 || keyNameSize != 8 {
		return nil, fmt.Errorf("[BLTE] Unexpected keyNameSize: %d", keyNameSize)
	}
	keyNameBytes := make([]string, keyNameSize)
	for i := 0; i < keyNameSize; i++ {
		keyNameBytes[i] = data.ReadHexString(1)
	}
	for i, j := 0, len(keyNameBytes)-1; i < j; i, j = i+1, j-1 {
		keyNameBytes[i], keyNameBytes[j] = keyNameBytes[j], keyNameBytes[i]
	}
	keyName := ""
	for _, part := range keyNameBytes {
		keyName += part
	}
	ivSize := int(data.ReadUInt8().(int64))
	if (ivSize != 4 && ivSize != 8) || ivSize > 8 {
		return nil, fmt.Errorf("[BLTE] Unexpected ivSize: %d", ivSize)
	}
	ivShort := data.ReadUInt8(ivSize).([]int64)
	if data.RemainingBytes() == 0 {
		return nil, fmt.Errorf("[BLTE] Unexpected end of data before encryption flag.")
	}
	encryptType := int(data.ReadUInt8().(int64))
	if encryptType != encTypeSalsa20 {
		return nil, fmt.Errorf("[BLTE] Unexpected encryption type: %d", encryptType)
	}
	for shift, i := 0, 0; i < 4; shift, i = shift+8, i+1 {
		ivShort[i] = int64((int(ivShort[i]) ^ ((index >> shift) & 0xFF)) & 0xFF)
	}
	key, ok := GetKey(keyName)
	if !ok {
		return nil, &EncryptionError{Key: keyName}
	}
	nonce := make([]byte, 8)
	for i := 0; i < 8; i++ {
		if i < len(ivShort) {
			nonce[i] = byte(ivShort[i])
		}
	}
	instance, err := NewSalsa20(nonce, key, 20)
	if err != nil {
		return nil, err
	}
	return instance.Process(data.ReadBuffer(buffer.ReadBufferOptions{
		Length: blockEnd - data.Offset(),
		Wrap:   true,
	}).(*buffer.Buffer)), nil
}

func (r *BLTEReader) writeBufferBLTE(buf *buffer.Buffer, blockEnd int) {
	copy(r.Raw()[r.Offset():], buf.Raw()[buf.Offset():blockEnd])
	r.SetOffset(r.Offset() + (blockEnd - buf.Offset()))
}

// CheckBounds ensures required BLTE blocks are processed before reads.
func (r *BLTEReader) CheckBounds(length int) {
	r.Buffer.CheckBounds(length)
	pos := r.Offset() + length
	for pos > r.blockWriteIndex && r.blockIndex < len(r.blocks) {
		if err := r.processBlock(); err != nil {
			panic(err)
		}
	}
}

// WriteToFile processes all blocks and writes decoded data to disk.
func (r *BLTEReader) WriteToFile(file string) error {
	if err := r.ProcessAllBlocks(); err != nil {
		return err
	}
	return r.Buffer.WriteToFile(file)
}

// ReadUInt8 overrides buffer reads to lazily process BLTE blocks.
func (r *BLTEReader) ReadUInt8(count ...int) any {
	if len(count) > 0 {
		r.CheckBounds(count[0])
	} else {
		r.CheckBounds(1)
	}
	return r.Buffer.ReadUInt8(count...)
}

// ReadBuffer overrides buffer reads to lazily process BLTE blocks.
func (r *BLTEReader) ReadBuffer(opts ...buffer.ReadBufferOptions) any {
	length := r.RemainingBytes()
	if len(opts) > 0 && opts[0].Length != 0 {
		length = opts[0].Length
	}
	r.CheckBounds(length)
	return r.Buffer.ReadBuffer(opts...)
}

// ReadString overrides buffer reads to lazily process BLTE blocks.
func (r *BLTEReader) ReadString(length int, encoding string) string {
	r.CheckBounds(length)
	return r.Buffer.ReadString(length, encoding)
}

// ReadUInt32LE overrides buffer reads to lazily process BLTE blocks.
func (r *BLTEReader) ReadUInt32LE(count ...int) any {
	n := 4
	if len(count) > 0 {
		n = 4 * count[0]
	} else {
		n = 4
	}
	r.CheckBounds(n)
	return r.Buffer.ReadUInt32LE(count...)
}

// ReadInt32LE overrides buffer reads to lazily process BLTE blocks.
func (r *BLTEReader) ReadInt32LE(count ...int) any {
	n := 4
	if len(count) > 0 {
		n = 4 * count[0]
	} else {
		n = 4
	}
	r.CheckBounds(n)
	return r.Buffer.ReadInt32LE(count...)
}

// ReadUInt16LE overrides buffer reads to lazily process BLTE blocks.
func (r *BLTEReader) ReadUInt16LE(count ...int) any {
	n := 2
	if len(count) > 0 {
		n = 2 * count[0]
	} else {
		n = 2
	}
	r.CheckBounds(n)
	return r.Buffer.ReadUInt16LE(count...)
}

// ReadBinaryKey overrides buffer reads to lazily process BLTE blocks.
func (r *BLTEReader) ReadBinaryKey(length int) string {
	r.CheckBounds(length)
	return r.Buffer.ReadBinaryKey(length)
}

// Move overrides buffer move to trigger bounds checks.
func (r *BLTEReader) Move(ofs int) {
	if ofs > 0 {
		r.CheckBounds(ofs)
	}
	r.Buffer.Move(ofs)
}

// RemainingBytes returns remaining decoded bytes, processing blocks as needed.
func (r *BLTEReader) RemainingBytes() int {
	for r.blockIndex < len(r.blocks) {
		if err := r.processBlock(); err != nil {
			panic(err)
		}
	}
	return r.Buffer.RemainingBytes()
}
