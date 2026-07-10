package wc3

import (
	"encoding/binary"
	"fmt"
	"math"
)

// W3Buffer is a little-endian binary reader/writer for WC3 map chunks.
type W3Buffer struct {
	data []byte
	pos  int
}

// NewW3Buffer creates a buffer from bytes.
func NewW3Buffer(data []byte) *W3Buffer {
	return &W3Buffer{data: append([]byte(nil), data...)}
}

// AllocW3Buffer allocates an empty buffer with capacity.
func AllocW3Buffer(capacity int) *W3Buffer {
	return &W3Buffer{data: make([]byte, 0, capacity)}
}

// Bytes returns written bytes.
func (w *W3Buffer) Bytes() []byte {
	return w.data
}

// ReadInt reads a little-endian int32.
func (w *W3Buffer) ReadInt() int32 {
	v, _ := w.readUint32()
	return int32(v)
}

// ReadShort reads a little-endian int16.
func (w *W3Buffer) ReadShort() int16 {
	if w.pos+2 > len(w.data) {
		return 0
	}
	v := int16(binary.LittleEndian.Uint16(w.data[w.pos:]))
	w.pos += 2
	return v
}

// ReadFloat reads a little-endian float32 rounded to 3 decimal places.
func (w *W3Buffer) ReadFloat() float32 {
	v, _ := w.ReadFloat32()
	return float32(math.Round(float64(v)*1000) / 1000)
}

// ReadString reads a null-terminated string.
func (w *W3Buffer) ReadString() string {
	start := w.pos
	for w.pos < len(w.data) && w.data[w.pos] != 0 {
		w.pos++
	}
	s := string(w.data[start:w.pos])
	if w.pos < len(w.data) {
		w.pos++ // consume null terminator
	}
	return s
}

// ReadChars reads len bytes as a string.
func (w *W3Buffer) ReadChars(length int) string {
	if length <= 0 {
		length = 1
	}
	if w.pos+length > len(w.data) {
		length = len(w.data) - w.pos
	}
	s := string(w.data[w.pos : w.pos+length])
	w.pos += length
	return s
}

// ReadByte reads one byte.
func (w *W3Buffer) ReadByte() byte {
	if w.pos >= len(w.data) {
		return 0
	}
	b := w.data[w.pos]
	w.pos++
	return b
}

// IsExhausted reports whether all bytes have been read.
func (w *W3Buffer) IsExhausted() bool {
	return w.pos >= len(w.data)
}

// ReadUint32 reads a little-endian uint32.
func (w *W3Buffer) ReadUint32() (uint32, error) {
	return w.readUint32()
}

func (w *W3Buffer) readUint32() (uint32, error) {
	if w.pos+4 > len(w.data) {
		return 0, fmt.Errorf("w3 buffer underflow")
	}
	v := binary.LittleEndian.Uint32(w.data[w.pos:])
	w.pos += 4
	return v, nil
}

// ReadFloat32 reads a little-endian float32.
func (w *W3Buffer) ReadFloat32() (float32, error) {
	if w.pos+4 > len(w.data) {
		return 0, fmt.Errorf("w3 buffer underflow")
	}
	bits := binary.LittleEndian.Uint32(w.data[w.pos:])
	w.pos += 4
	return math.Float32frombits(bits), nil
}

// WriteUint32 appends a little-endian uint32.
func (w *W3Buffer) WriteUint32(v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	w.data = append(w.data, buf[:]...)
}

// WriteFloat32 appends a little-endian float32.
func (w *W3Buffer) WriteFloat32(v float32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(v))
	w.data = append(w.data, buf[:]...)
}

// WriteString appends a null-terminated string.
func (w *W3Buffer) WriteString(s string) {
	w.data = append(w.data, []byte(s)...)
	w.data = append(w.data, 0)
}
