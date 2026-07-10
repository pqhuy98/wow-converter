package mdx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
)

// BinaryStream is a little-endian binary reader/writer over a byte slice.
type BinaryStream struct {
	data      []byte
	index     int
	byteLength int
	remaining  int
}

// NewBinaryStream wraps buf for reading or writing.
func NewBinaryStream(buf []byte) *BinaryStream {
	return &BinaryStream{
		data:       buf,
		byteLength: len(buf),
		remaining:  len(buf),
	}
}

// NewBinaryStreamWithCapacity allocates a buffer of the given size for writing.
func NewBinaryStreamWithCapacity(size int) *BinaryStream {
	return NewBinaryStream(make([]byte, size))
}

// Bytes returns the underlying buffer.
func (s *BinaryStream) Bytes() []byte {
	return s.data
}

// Index returns the current read/write position.
func (s *BinaryStream) Index() int {
	return s.index
}

// Substream creates a sub-reader at the current position.
func (s *BinaryStream) Substream(byteLength int) (*BinaryStream, error) {
	if s.remaining < byteLength {
		return nil, errors.New("ByteStream: substream: premature end")
	}
	index := s.index
	s.index += byteLength
	s.remaining -= byteLength
	return NewBinaryStream(s.data[index : index+byteLength]), nil
}

// Skip advances the stream by n bytes.
func (s *BinaryStream) Skip(n int) error {
	if s.remaining < n {
		return errors.New("ByteStream: skip: premature end")
	}
	s.index += n
	s.remaining -= n
	return nil
}

// Seek sets the stream index.
func (s *BinaryStream) Seek(index int) {
	s.index = index
	s.remaining = s.byteLength - index
}

func boundIndexOf(data []byte, b byte, start, maxLen int) int {
	end := start + maxLen
	if end > len(data) {
		end = len(data)
	}
	for i := start; i < end; i++ {
		if data[i] == b {
			return i
		}
	}
	return -1
}

// Read reads a fixed-size UTF-8 string, stopping at the first NULL within the block.
func (s *BinaryStream) Read(n int) (string, error) {
	if s.remaining < n {
		return "", errors.New("ByteStream: read: premature end")
	}
	start := s.index
	end := boundIndexOf(s.data, 0, start, n)
	if end == -1 {
		end = start + n
	}
	s.index += n
	s.remaining -= n
	return string(s.data[start:end]), nil
}

// ReadNull reads a NULL-terminated UTF-8 string.
func (s *BinaryStream) ReadNull() (string, error) {
	if s.remaining < 1 {
		return "", errors.New("ByteStream: readNull: premature end")
	}
	start := s.index
	end := bytes.IndexByte(s.data[start:], 0)
	if end == -1 {
		end = len(s.data) - 1 - start
	}
	n := end + 1
	s.index += n
	s.remaining -= n
	return string(s.data[start : start+end]), nil
}

// ReadBinary reads n bytes as a Latin-1 string.
func (s *BinaryStream) ReadBinary(n int) (string, error) {
	if s.remaining < n {
		return "", errors.New("ByteStream: readBinary: premature end")
	}
	b := s.data[s.index : s.index+n]
	s.index += n
	s.remaining -= n
	return string(b), nil
}

func (s *BinaryStream) ReadInt8() (int8, error) {
	if s.remaining < 1 {
		return 0, errors.New("ByteStream: readInt8: premature end")
	}
	v := int8(s.data[s.index])
	s.index++
	s.remaining--
	return v, nil
}

func (s *BinaryStream) ReadInt16() (int16, error) {
	if s.remaining < 2 {
		return 0, errors.New("ByteStream: readInt16: premature end")
	}
	v := int16(binary.LittleEndian.Uint16(s.data[s.index:]))
	s.index += 2
	s.remaining -= 2
	return v, nil
}

func (s *BinaryStream) ReadInt32() (int32, error) {
	if s.remaining < 4 {
		return 0, errors.New("ByteStream: readInt32: premature end")
	}
	v := int32(binary.LittleEndian.Uint32(s.data[s.index:]))
	s.index += 4
	s.remaining -= 4
	return v, nil
}

func (s *BinaryStream) ReadUint8() (uint8, error) {
	if s.remaining < 1 {
		return 0, errors.New("ByteStream: readUint8: premature end")
	}
	v := s.data[s.index]
	s.index++
	s.remaining--
	return v, nil
}

func (s *BinaryStream) ReadUint16() (uint16, error) {
	if s.remaining < 2 {
		return 0, errors.New("ByteStream: readUint16: premature end")
	}
	v := binary.LittleEndian.Uint16(s.data[s.index:])
	s.index += 2
	s.remaining -= 2
	return v, nil
}

func (s *BinaryStream) ReadUint32() (uint32, error) {
	if s.remaining < 4 {
		return 0, errors.New("ByteStream: readUint32: premature end")
	}
	v := binary.LittleEndian.Uint32(s.data[s.index:])
	s.index += 4
	s.remaining -= 4
	return v, nil
}

func (s *BinaryStream) ReadFloat32() (float32, error) {
	if s.remaining < 4 {
		return 0, errors.New("ByteStream: readFloat32: premature end")
	}
	v := math.Float32frombits(binary.LittleEndian.Uint32(s.data[s.index:]))
	s.index += 4
	s.remaining -= 4
	return v, nil
}

func (s *BinaryStream) ReadFloat64() (float64, error) {
	if s.remaining < 8 {
		return 0, errors.New("ByteStream: readFloat64: premature end")
	}
	v := math.Float64frombits(binary.LittleEndian.Uint64(s.data[s.index:]))
	s.index += 8
	s.remaining -= 8
	return v, nil
}

func (s *BinaryStream) readSlice(n int) ([]byte, error) {
	if s.remaining < n {
		return nil, errors.New("ByteStream: read: premature end")
	}
	out := make([]byte, n)
	copy(out, s.data[s.index:s.index+n])
	s.index += n
	s.remaining -= n
	return out, nil
}

// ReadInt8Array reads count int8 values.
func (s *BinaryStream) ReadInt8Array(count int) ([]int8, error) {
	raw, err := s.readSlice(count)
	if err != nil {
		return nil, err
	}
	out := make([]int8, count)
	for i, b := range raw {
		out[i] = int8(b)
	}
	return out, nil
}

// ReadInt16Array reads count int16 values.
func (s *BinaryStream) ReadInt16Array(count int) ([]int16, error) {
	if s.remaining < count*2 {
		return nil, errors.New("ByteStream: readInt16Array: premature end")
	}
	out := make([]int16, count)
	for i := 0; i < count; i++ {
		out[i] = int16(binary.LittleEndian.Uint16(s.data[s.index:]))
		s.index += 2
	}
	s.remaining -= count * 2
	return out, nil
}

// ReadInt32Array reads count int32 values into dst or a new slice.
func (s *BinaryStream) ReadInt32Array(count int) ([]int32, error) {
	if s.remaining < count*4 {
		return nil, errors.New("ByteStream: readInt32Array: premature end")
	}
	out := make([]int32, count)
	for i := 0; i < count; i++ {
		out[i] = int32(binary.LittleEndian.Uint32(s.data[s.index:]))
		s.index += 4
	}
	s.remaining -= count * 4
	return out, nil
}

// ReadUint8Array reads count bytes.
func (s *BinaryStream) ReadUint8Array(count int) ([]uint8, error) {
	raw, err := s.readSlice(count)
	if err != nil {
		return nil, err
	}
	out := make([]uint8, count)
	copy(out, raw)
	return out, nil
}

// ReadUint16Array reads count uint16 values.
func (s *BinaryStream) ReadUint16Array(count int) ([]uint16, error) {
	if s.remaining < count*2 {
		return nil, errors.New("ByteStream: readUint16Array: premature end")
	}
	out := make([]uint16, count)
	for i := 0; i < count; i++ {
		out[i] = binary.LittleEndian.Uint16(s.data[s.index:])
		s.index += 2
	}
	s.remaining -= count * 2
	return out, nil
}

// ReadUint32Array reads count uint32 values.
func (s *BinaryStream) ReadUint32Array(count int) ([]uint32, error) {
	if s.remaining < count*4 {
		return nil, errors.New("ByteStream: readUint32Array: premature end")
	}
	out := make([]uint32, count)
	for i := 0; i < count; i++ {
		out[i] = binary.LittleEndian.Uint32(s.data[s.index:])
		s.index += 4
	}
	s.remaining -= count * 4
	return out, nil
}

// ReadFloat32Array reads count float32 values.
func (s *BinaryStream) ReadFloat32Array(count int) ([]float32, error) {
	if s.remaining < count*4 {
		return nil, errors.New("ByteStream: readFloat32Array: premature end")
	}
	out := make([]float32, count)
	for i := 0; i < count; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(s.data[s.index:]))
		s.index += 4
	}
	s.remaining -= count * 4
	return out, nil
}

// ReadFloat64Array reads count float64 values.
func (s *BinaryStream) ReadFloat64Array(count int) ([]float64, error) {
	if s.remaining < count*8 {
		return nil, errors.New("ByteStream: readFloat64Array: premature end")
	}
	out := make([]float64, count)
	for i := 0; i < count; i++ {
		out[i] = math.Float64frombits(binary.LittleEndian.Uint64(s.data[s.index:]))
		s.index += 8
	}
	s.remaining -= count * 8
	return out, nil
}

// Write writes UTF-8 bytes and returns the number written.
func (s *BinaryStream) Write(utf8 string) int {
	n := copy(s.data[s.index:], []byte(utf8))
	s.index += n
	return n
}

// WriteNull writes a UTF-8 string followed by a NULL byte.
func (s *BinaryStream) WriteNull(utf8 string) int {
	n := s.Write(utf8)
	s.data[s.index] = 0
	s.index++
	return n + 1
}

// WriteBinary writes a Latin-1 string.
func (s *BinaryStream) WriteBinary(value string) {
	n := copy(s.data[s.index:], []byte(value))
	s.index += n
}

func (s *BinaryStream) WriteInt8(v int8) {
	s.data[s.index] = uint8(v)
	s.index++
}

func (s *BinaryStream) WriteInt16(v int16) {
	binary.LittleEndian.PutUint16(s.data[s.index:], uint16(v))
	s.index += 2
}

func (s *BinaryStream) WriteInt32(v int32) {
	binary.LittleEndian.PutUint32(s.data[s.index:], uint32(v))
	s.index += 4
}

func (s *BinaryStream) WriteUint8(v uint8) {
	s.data[s.index] = v
	s.index++
}

func (s *BinaryStream) WriteUint16(v uint16) {
	binary.LittleEndian.PutUint16(s.data[s.index:], v)
	s.index += 2
}

func (s *BinaryStream) WriteUint32(v uint32) {
	binary.LittleEndian.PutUint32(s.data[s.index:], uint32(v))
	s.index += 4
}

func (s *BinaryStream) WriteFloat32(v float32) {
	binary.LittleEndian.PutUint32(s.data[s.index:], math.Float32bits(v))
	s.index += 4
}

func (s *BinaryStream) WriteFloat64(v float64) {
	binary.LittleEndian.PutUint64(s.data[s.index:], math.Float64bits(v))
	s.index += 8
}

func (s *BinaryStream) WriteInt8Array(view []int8) {
	for i, v := range view {
		s.data[s.index+i] = uint8(v)
	}
	s.index += len(view)
}

func (s *BinaryStream) WriteInt16Array(view []int16) {
	for _, v := range view {
		binary.LittleEndian.PutUint16(s.data[s.index:], uint16(v))
		s.index += 2
	}
}

func (s *BinaryStream) WriteInt32Array(view []int32) {
	for _, v := range view {
		binary.LittleEndian.PutUint32(s.data[s.index:], uint32(v))
		s.index += 4
	}
}

func (s *BinaryStream) WriteUint8Array(view []uint8) {
	copy(s.data[s.index:], view)
	s.index += len(view)
}

func (s *BinaryStream) WriteUint16Array(view []uint16) {
	for _, v := range view {
		binary.LittleEndian.PutUint16(s.data[s.index:], v)
		s.index += 2
	}
}

func (s *BinaryStream) WriteUint32Array(view []uint32) {
	for _, v := range view {
		binary.LittleEndian.PutUint32(s.data[s.index:], v)
		s.index += 4
	}
}

func (s *BinaryStream) WriteFloat32Array(view []float32) {
	for _, v := range view {
		binary.LittleEndian.PutUint32(s.data[s.index:], math.Float32bits(v))
		s.index += 4
	}
}

func (s *BinaryStream) WriteFloat64Array(view []float64) {
	for _, v := range view {
		binary.LittleEndian.PutUint64(s.data[s.index:], math.Float64bits(v))
		s.index += 8
	}
}
