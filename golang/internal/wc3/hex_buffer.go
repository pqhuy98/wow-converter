package wc3

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
)

// HexBuffer accumulates little-endian binary data for WC3 map files.
type HexBuffer struct {
	data []byte
	pos  int
}

// NewHexBufferWriter creates an empty write buffer.
func NewHexBufferWriter() *HexBuffer {
	return &HexBuffer{}
}

// NewHexBuffer decodes a hex string into a read buffer.
func NewHexBuffer(hexStr string) (*HexBuffer, error) {
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("hex decode: %w", err)
	}
	return &HexBuffer{data: data}, nil
}

// FromBytes wraps raw bytes for reading.
func FromBytes(data []byte) *HexBuffer {
	return &HexBuffer{data: append([]byte(nil), data...)}
}

// GetBuffer returns accumulated bytes (write mode).
func (h *HexBuffer) GetBuffer() []byte {
	return h.data
}

// AddString appends a null-terminated UTF-8 string.
func (h *HexBuffer) AddString(s string) {
	h.AddStringNoNewline(s)
	h.AddNullTerminator()
}

// AddStringNoNewline appends a UTF-8 string without a null terminator.
func (h *HexBuffer) AddStringNoNewline(s string) {
	h.data = append(h.data, []byte(s)...)
}

// AddNullTerminator appends a zero byte.
func (h *HexBuffer) AddNullTerminator() {
	h.data = append(h.data, 0)
}

// AddChar appends one ASCII character byte.
func (h *HexBuffer) AddChar(char string) {
	if len(char) == 0 {
		return
	}
	h.data = append(h.data, char[0])
}

// AddChars appends each character byte from s.
func (h *HexBuffer) AddChars(chars string) {
	for i := 0; i < len(chars); i++ {
		h.data = append(h.data, chars[i])
	}
}

// AddInt appends a little-endian int32 (or int16 when isShort is true).
func (h *HexBuffer) AddInt(v int, isShort ...bool) {
	if len(isShort) > 0 && isShort[0] {
		h.AddShort(int16(v))
		return
	}
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(int32(v)))
	h.data = append(h.data, buf[:]...)
}

// AddShort appends a little-endian int16.
func (h *HexBuffer) AddShort(v int16) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], uint16(v))
	h.data = append(h.data, buf[:]...)
}

// AddFloat appends a little-endian float32.
func (h *HexBuffer) AddFloat(v float32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(v))
	h.data = append(h.data, buf[:]...)
}

// AddByte appends one byte.
func (h *HexBuffer) AddByte(v byte) {
	h.data = append(h.data, v)
}

// Remaining returns unread byte count (read mode).
func (h *HexBuffer) Remaining() int {
	return len(h.data) - h.pos
}

// ReadByte reads one byte (read mode).
func (h *HexBuffer) ReadByte() (byte, error) {
	if h.pos >= len(h.data) {
		return 0, fmt.Errorf("hex buffer underflow")
	}
	b := h.data[h.pos]
	h.pos++
	return b, nil
}

// ReadBytes reads n bytes (read mode).
func (h *HexBuffer) ReadBytes(n int) ([]byte, error) {
	if h.pos+n > len(h.data) {
		return nil, fmt.Errorf("hex buffer underflow")
	}
	out := h.data[h.pos : h.pos+n]
	h.pos += n
	return out, nil
}

// Data returns the full underlying buffer.
func (h *HexBuffer) Data() []byte {
	return h.data
}
