package png

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
)

// Writer encodes RGBA8 images as PNG.
type Writer struct {
	width, height int
	pixels        []byte
}

// NewWriter creates a PNG writer.
func NewWriter(width, height int) *Writer {
	return &Writer{
		width: width, height: height,
		pixels: make([]byte, width*height*4),
	}
}

// Pixels returns the writable RGBA pixel buffer.
func (w *Writer) Pixels() []byte { return w.pixels }

// Encode returns PNG file bytes.
func (w *Writer) Encode() ([]byte, error) {
	var out bytes.Buffer
	writeChunk := func(tag string, data []byte) {
		binary.Write(&out, binary.BigEndian, uint32(len(data)))
		out.WriteString(tag)
		out.Write(data)
		crc := crc32.ChecksumIEEE(append([]byte(tag), data...))
		binary.Write(&out, binary.BigEndian, crc)
	}

	out.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w.width))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(w.height))
	ihdr[8] = 8
	ihdr[9] = 6
	writeChunk("IHDR", ihdr)

	rowBytes := w.width * 4
	raw := make([]byte, (rowBytes+1)*w.height)
	for y := 0; y < w.height; y++ {
		raw[y*(rowBytes+1)] = 0
		copy(raw[y*(rowBytes+1)+1:], w.pixels[y*rowBytes:(y+1)*rowBytes])
	}
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	writeChunk("IDAT", compressed.Bytes())
	writeChunk("IEND", nil)
	return out.Bytes(), nil
}
