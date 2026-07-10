package buffer

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
)

// Buffer provides binary read/write helpers over a byte slice.
type Buffer struct {
	ofs int
	buf []byte
}

// Alloc allocates a buffer with the given length.
func Alloc(length int, secure bool) *Buffer {
	var buf []byte
	if secure {
		buf = make([]byte, length)
	} else {
		buf = make([]byte, length)
	}
	return &Buffer{buf: buf}
}

// From wraps an existing byte slice.
func From(source []byte) *Buffer {
	return &Buffer{buf: append([]byte(nil), source...)}
}

// FromBase64 decodes a base64 string into a buffer.
func FromBase64(source string) (*Buffer, error) {
	data, err := base64.StdEncoding.DecodeString(source)
	if err != nil {
		return nil, err
	}
	return &Buffer{buf: data}, nil
}

// Concat concatenates buffers into a single buffer.
func Concat(buffers []*Buffer) *Buffer {
	var parts [][]byte
	for _, b := range buffers {
		parts = append(parts, b.buf)
	}
	return &Buffer{buf: bytes.Join(parts, nil)}
}

// ReadFile loads a file from disk into a wrapped buffer.
func ReadFile(file string) (*Buffer, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return &Buffer{buf: data}, nil
}

// NewBuffer wraps an existing byte slice without copying.
func NewBuffer(buf []byte) *Buffer {
	return &Buffer{buf: buf}
}

func (b *Buffer) ByteLength() int { return len(b.buf) }
func (b *Buffer) RemainingBytes() int { return len(b.buf) - b.ofs }
func (b *Buffer) Offset() int { return b.ofs }
func (b *Buffer) Raw() []byte { return b.buf }

func (b *Buffer) Seek(ofs int) {
	pos := ofs
	if ofs < 0 {
		pos = len(b.buf) + ofs
	}
	if pos < 0 || pos > len(b.buf) {
		panic(fmt.Sprintf("seek() offset out of bounds %d -> %d ! %d", ofs, pos, len(b.buf)))
	}
	b.ofs = pos
}

func (b *Buffer) Move(ofs int) {
	pos := b.ofs + ofs
	if pos < 0 || pos > len(b.buf) {
		panic(fmt.Sprintf("move() offset out of bounds %d -> %d ! %d", ofs, pos, len(b.buf)))
	}
	b.ofs = pos
}

func (b *Buffer) checkBounds(length int) {
	if b.RemainingBytes() < length {
		panic(fmt.Sprintf("Buffer operation out-of-bounds: %d > %d", length, b.RemainingBytes()))
	}
}

func readSignedLE(buf []byte, ofs, byteLength int) int64 {
	var val uint64
	for i := 0; i < byteLength; i++ {
		val |= uint64(buf[ofs+i]) << (8 * i)
	}
	shift := 64 - byteLength*8
	return int64(int64(val<<uint(shift)) >> uint(shift))
}

func readUnsignedLE(buf []byte, ofs, byteLength int) uint64 {
	var val uint64
	for i := 0; i < byteLength; i++ {
		val |= uint64(buf[ofs+i]) << (8 * i)
	}
	return val
}

func readSignedBE(buf []byte, ofs, byteLength int) int64 {
	var val uint64
	for i := 0; i < byteLength; i++ {
		val = (val << 8) | uint64(buf[ofs+i])
	}
	shift := 64 - byteLength*8
	return int64(int64(val<<uint(shift)) >> uint(shift))
}

func readUnsignedBE(buf []byte, ofs, byteLength int) uint64 {
	var val uint64
	for i := 0; i < byteLength; i++ {
		val = (val << 8) | uint64(buf[ofs+i])
	}
	return val
}

func (b *Buffer) readInts(count *int, signed bool, le bool, byteLength int) any {
	if count != nil {
		b.checkBounds(byteLength * *count)
		values := make([]int64, *count)
		for i := 0; i < *count; i++ {
			if signed {
				if le {
					values[i] = readSignedLE(b.buf, b.ofs, byteLength)
				} else {
					values[i] = readSignedBE(b.buf, b.ofs, byteLength)
				}
			} else {
				if le {
					values[i] = int64(readUnsignedLE(b.buf, b.ofs, byteLength))
				} else {
					values[i] = int64(readUnsignedBE(b.buf, b.ofs, byteLength))
				}
			}
			b.ofs += byteLength
		}
		return values
	}
	b.checkBounds(byteLength)
	var value int64
	if signed {
		if le {
			value = readSignedLE(b.buf, b.ofs, byteLength)
		} else {
			value = readSignedBE(b.buf, b.ofs, byteLength)
		}
	} else {
		if le {
			value = int64(readUnsignedLE(b.buf, b.ofs, byteLength))
		} else {
			value = int64(readUnsignedBE(b.buf, b.ofs, byteLength))
		}
	}
	b.ofs += byteLength
	return value
}

func (b *Buffer) readBigInts(count *int, le bool) any {
	if count != nil {
		b.checkBounds(8 * *count)
		values := make([]uint64, *count)
		for i := 0; i < *count; i++ {
			if le {
				values[i] = binary.LittleEndian.Uint64(b.buf[b.ofs:])
			} else {
				values[i] = binary.BigEndian.Uint64(b.buf[b.ofs:])
			}
			b.ofs += 8
		}
		return values
	}
	b.checkBounds(8)
	var value uint64
	if le {
		value = binary.LittleEndian.Uint64(b.buf[b.ofs:])
	} else {
		value = binary.BigEndian.Uint64(b.buf[b.ofs:])
	}
	b.ofs += 8
	return value
}

func (b *Buffer) ReadIntLE(byteLength int, count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, true, byteLength)
}

func (b *Buffer) ReadUIntLE(byteLength int, count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, true, byteLength)
}

func (b *Buffer) ReadIntBE(byteLength int, count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, false, byteLength)
}

func (b *Buffer) ReadUIntBE(byteLength int, count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, false, byteLength)
}

func (b *Buffer) ReadInt8(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, true, 1)
}

func (b *Buffer) ReadUInt8(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, true, 1)
}

func (b *Buffer) ReadInt16LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, true, 2)
}

func (b *Buffer) ReadUInt16LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, true, 2)
}

func (b *Buffer) ReadInt16BE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, false, 2)
}

func (b *Buffer) ReadUInt16BE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, false, 2)
}

func (b *Buffer) ReadInt24LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, true, 3)
}

func (b *Buffer) ReadUInt24LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, true, 3)
}

func (b *Buffer) ReadInt24BE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, false, 3)
}

func (b *Buffer) ReadUInt24BE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, false, 3)
}

func (b *Buffer) ReadInt32LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, true, 4)
}

func (b *Buffer) ReadUInt32LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, true, 4)
}

func (b *Buffer) ReadInt32BE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, false, 4)
}

func (b *Buffer) ReadUInt32BE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, false, 4)
}

func (b *Buffer) ReadInt40LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, true, 5)
}

func (b *Buffer) ReadUInt40LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, true, 5)
}

func (b *Buffer) ReadInt40BE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, false, 5)
}

func (b *Buffer) ReadUInt40BE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, false, 5)
}

func (b *Buffer) ReadInt48LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, true, true, 6)
}

func (b *Buffer) ReadUInt48LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readInts(c, false, true, 6)
}

func (b *Buffer) ReadInt64LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readSignedBigInts(c, true)
}

func (b *Buffer) readSignedBigInts(count *int, le bool) any {
	if count != nil {
		b.checkBounds(8 * *count)
		values := make([]int64, *count)
		for i := 0; i < *count; i++ {
			var u uint64
			if le {
				u = binary.LittleEndian.Uint64(b.buf[b.ofs:])
			} else {
				u = binary.BigEndian.Uint64(b.buf[b.ofs:])
			}
			values[i] = int64(u)
			b.ofs += 8
		}
		return values
	}
	b.checkBounds(8)
	var u uint64
	if le {
		u = binary.LittleEndian.Uint64(b.buf[b.ofs:])
	} else {
		u = binary.BigEndian.Uint64(b.buf[b.ofs:])
	}
	b.ofs += 8
	return int64(u)
}

func (b *Buffer) ReadUInt64LE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readBigInts(c, true)
}

func (b *Buffer) ReadInt64BE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readSignedBigInts(c, false)
}

func (b *Buffer) ReadUInt64BE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readBigInts(c, false)
}

func (b *Buffer) readFloats(count *int, le bool) any {
	if count != nil {
		b.checkBounds(4 * *count)
		values := make([]float32, *count)
		for i := 0; i < *count; i++ {
			var bits uint32
			if le {
				bits = binary.LittleEndian.Uint32(b.buf[b.ofs:])
			} else {
				bits = binary.BigEndian.Uint32(b.buf[b.ofs:])
			}
			values[i] = math.Float32frombits(bits)
			b.ofs += 4
		}
		return values
	}
	b.checkBounds(4)
	var bits uint32
	if le {
		bits = binary.LittleEndian.Uint32(b.buf[b.ofs:])
	} else {
		bits = binary.BigEndian.Uint32(b.buf[b.ofs:])
	}
	b.ofs += 4
	return math.Float32frombits(bits)
}

func (b *Buffer) ReadFloatLE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readFloats(c, true)
}

func (b *Buffer) ReadFloatBE(count ...int) any {
	var c *int
	if len(count) > 0 {
		c = &count[0]
	}
	return b.readFloats(c, false)
}

func (b *Buffer) ReadDoubleLE(count ...int) any {
	if len(count) > 0 {
		b.checkBounds(8 * count[0])
		values := make([]float64, count[0])
		for i := 0; i < count[0]; i++ {
			values[i] = math.Float64frombits(binary.LittleEndian.Uint64(b.buf[b.ofs:]))
			b.ofs += 8
		}
		return values
	}
	b.checkBounds(8)
	value := math.Float64frombits(binary.LittleEndian.Uint64(b.buf[b.ofs:]))
	b.ofs += 8
	return value
}

func (b *Buffer) ReadHexString(length int) string {
	b.checkBounds(length)
	hexStr := hex.EncodeToString(b.buf[b.ofs : b.ofs+length])
	b.ofs += length
	return hexStr
}

func (b *Buffer) ReadBinaryKey(length int) string {
	b.checkBounds(length)
	key := string(b.buf[b.ofs : b.ofs+length])
	b.ofs += length
	return key
}

type ReadBufferOptions struct {
	Length  int
	Wrap    bool
	Inflate bool
}

func (b *Buffer) ReadBuffer(opts ...ReadBufferOptions) any {
	length := b.RemainingBytes()
	wrap := true
	inflate := false
	if len(opts) > 0 {
		if opts[0].Length != 0 {
			length = opts[0].Length
		}
		wrap = opts[0].Wrap
		inflate = opts[0].Inflate
	}
	b.checkBounds(length)
	buf := make([]byte, length)
	copy(buf, b.buf[b.ofs:b.ofs+length])
	b.ofs += length
	if inflate {
		r, err := zlib.NewReader(bytes.NewReader(buf))
		if err != nil {
			panic(err)
		}
		inflated, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			panic(err)
		}
		buf = inflated
	}
	if wrap {
		return &Buffer{buf: buf}
	}
	return buf
}

func (b *Buffer) ReadString(length int, encoding string) string {
	if length == 0 {
		return ""
	}
	b.checkBounds(length)
	s := string(b.buf[b.ofs : b.ofs+length])
	if encoding != "" && encoding != "utf8" && encoding != "utf-8" && encoding != "latin1" {
		// Node supports many encodings; headless port only uses utf8/latin1.
	}
	b.ofs += length
	return s
}

func (b *Buffer) ReadNullTerminatedString(encoding string) string {
	startPos := b.ofs
	length := 0
	for b.RemainingBytes() > 0 {
		if b.ReadUInt8().(int64) == 0 {
			break
		}
		length++
	}
	b.Seek(startPos)
	str := b.ReadString(length, encoding)
	b.Move(1)
	return str
}

func (b *Buffer) StartsWith(input any, encoding string) bool {
	b.Seek(0)
	switch v := input.(type) {
	case string:
		return b.ReadString(len(v), encoding) == v
	case []string:
		for _, entry := range v {
			b.Seek(0)
			if b.ReadString(len(entry), encoding) == entry {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (b *Buffer) ReadJSON(length int, encoding string) any {
	if length == 0 {
		length = b.RemainingBytes()
	}
	var out any
	if err := json.Unmarshal([]byte(b.ReadString(length, encoding)), &out); err != nil {
		panic(err)
	}
	return out
}

var lineBreakPattern = regexp.MustCompile(`\r\n|\n|\r`)

func (b *Buffer) ReadLines(encoding string) []string {
	ofs := b.ofs
	b.Seek(0)
	str := b.ReadString(b.RemainingBytes(), encoding)
	b.Seek(ofs)
	// Preserve empty lines — DBD chunk separators depend on blank lines (matches TS split).
	return lineBreakPattern.Split(str, -1)
}

func (b *Buffer) Fill(value byte, length int) {
	if length == 0 {
		length = b.RemainingBytes()
	}
	b.checkBounds(length)
	for i := b.ofs; i < b.ofs+length; i++ {
		b.buf[i] = value
	}
	b.ofs += length
}

func (b *Buffer) writeInt(value int64, signed bool, le bool, byteLength int) {
	b.checkBounds(byteLength)
	if signed {
		if le {
			for i := 0; i < byteLength; i++ {
				b.buf[b.ofs+i] = byte(value >> (8 * i))
			}
		} else {
			for i := byteLength - 1; i >= 0; i-- {
				b.buf[b.ofs+(byteLength-1-i)] = byte(value >> (8 * i))
			}
		}
	} else {
		uv := uint64(value)
		if le {
			for i := 0; i < byteLength; i++ {
				b.buf[b.ofs+i] = byte(uv >> (8 * i))
			}
		} else {
			for i := byteLength - 1; i >= 0; i-- {
				b.buf[b.ofs+(byteLength-1-i)] = byte(uv >> (8 * i))
			}
		}
	}
	b.ofs += byteLength
}

func (b *Buffer) WriteInt8(value int64)   { b.writeInt(value, true, true, 1) }
func (b *Buffer) WriteUInt8(value int64)  { b.writeInt(value, false, true, 1) }
func (b *Buffer) WriteInt16LE(value int64)  { b.writeInt(value, true, true, 2) }
func (b *Buffer) WriteUInt16LE(value int64) { b.writeInt(value, false, true, 2) }
func (b *Buffer) WriteInt32LE(value int64)  { b.writeInt(value, true, true, 4) }
func (b *Buffer) WriteUInt32LE(value int64) { b.writeInt(value, false, true, 4) }
func (b *Buffer) WriteInt32BE(value int64)  { b.writeInt(value, true, false, 4) }
func (b *Buffer) WriteUInt32BE(value int64) { b.writeInt(value, false, false, 4) }

func (b *Buffer) WriteBigInt64LE(value uint64) {
	b.checkBounds(8)
	binary.LittleEndian.PutUint64(b.buf[b.ofs:], value)
	b.ofs += 8
}

func (b *Buffer) WriteBigUInt64LE(value uint64) {
	b.WriteBigInt64LE(value)
}

func (b *Buffer) WriteFloatLE(value float32) {
	b.checkBounds(4)
	binary.LittleEndian.PutUint32(b.buf[b.ofs:], math.Float32bits(value))
	b.ofs += 4
}

func (b *Buffer) WriteBuffer(buf any, copyLength int) {
	var startIndex int
	var rawBuf []byte
	length := copyLength

	switch v := buf.(type) {
	case *Buffer:
		startIndex = v.ofs
		if length == 0 {
			length = v.RemainingBytes()
		} else {
			v.checkBounds(length)
		}
		rawBuf = v.buf
	case []byte:
		if length == 0 {
			length = len(v)
		}
		rawBuf = v
	default:
		panic("WriteBuffer: unsupported buffer type")
	}

	b.checkBounds(length)
	copy(b.buf[b.ofs:], rawBuf[startIndex:startIndex+length])
	b.ofs += length
	if bw, ok := buf.(*Buffer); ok {
		bw.ofs += length
	}
}

func (b *Buffer) WriteToFile(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, b.buf, 0o644)
}

func (b *Buffer) IndexOfChar(char string, start int) int {
	if len(char) > 1 {
		panic("Buffer.IndexOfChar() given string, expected single character.")
	}
	return b.IndexOf(int(char[0]), start)
}

func (b *Buffer) IndexOf(byteVal int, start int) int {
	resetPos := b.ofs
	b.Seek(start)
	for b.RemainingBytes() > 0 {
		mark := b.ofs
		if b.ReadUInt8().(int64) == int64(byteVal) {
			b.Seek(resetPos)
			return mark
		}
	}
	b.Seek(resetPos)
	return -1
}

func (b *Buffer) ToBase64() string {
	return base64.StdEncoding.EncodeToString(b.buf)
}

func (b *Buffer) SetCapacity(capacity int, secure bool) {
	if capacity == len(b.buf) {
		return
	}
	var buf []byte
	if secure {
		buf = make([]byte, capacity)
	} else {
		buf = make([]byte, capacity)
	}
	copyLen := capacity
	if copyLen > len(b.buf) {
		copyLen = len(b.buf)
	}
	copy(buf, b.buf[:copyLen])
	b.buf = buf
}

func (b *Buffer) CalculateHash(hash string, encoding string) string {
	var sum []byte
	switch hash {
	case "md5":
		h := md5.Sum(b.buf)
		sum = h[:]
	case "sha1":
		h := sha1.Sum(b.buf)
		sum = h[:]
	default:
		panic("unsupported hash: " + hash)
	}
	if encoding == "base64" {
		return base64.StdEncoding.EncodeToString(sum)
	}
	return hex.EncodeToString(sum)
}

func (b *Buffer) IsZeroed() bool {
	for i := 0; i < len(b.buf); i++ {
		if b.buf[i] != 0 {
			return false
		}
	}
	return true
}

func (b *Buffer) Deflate() (*Buffer, error) {
	var out bytes.Buffer
	w := zlib.NewWriter(&out)
	if _, err := w.Write(b.buf); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return &Buffer{buf: out.Bytes()}, nil
}

func (b *Buffer) GetCRC32() uint32 {
	return CRC32(b.buf)
}

// CheckBounds exposes bounds checking for embedded types.
func (b *Buffer) CheckBounds(length int) {
	b.checkBounds(length)
}

// SetOffset sets the offset directly (for internal BLTE writing).
func (b *Buffer) SetOffset(ofs int) {
	b.ofs = ofs
}

// SetBuf replaces the internal buffer.
func (b *Buffer) SetBuf(buf []byte) {
	b.buf = buf
}
