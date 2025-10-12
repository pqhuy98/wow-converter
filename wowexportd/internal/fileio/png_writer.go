package fileio

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
)

// PNGWriter mirrors wow.export/src/js/png-writer.js behavior sufficiently to
// produce RGBA PNGs with adaptive per-scanline filtering and 8-bit depth.
// It does not attempt to bit-for-bit match the deflate stream, but matches
// chunk layout (IHDR, IDAT, IEND) and filtering/bit-depth settings.
type PNGWriter struct {
	Width         int
	Height        int
	bytesPerPixel int
	bitDepth      uint8
	colorType     uint8
	data          []byte
}

func NewPNGWriter(width, height int) *PNGWriter {
	return &PNGWriter{
		Width:         width,
		Height:        height,
		bytesPerPixel: 4,
		bitDepth:      8,
		colorType:     6, // RGBA
		data:          make([]byte, width*height*4),
	}
}

func (w *PNGWriter) PixelData() []byte { return w.data }

// getFilteredRaw applies adaptive filtering and returns the raw scanline buffer
// with a leading filter byte per scanline.
func (w *PNGWriter) getFilteredRaw() []byte {
	byteWidth := w.Width * w.bytesPerPixel
	raw := make([]byte, (byteWidth+1)*w.Height)

	// rolling buffer for computing sums quickly uses the destination buffer itself
	for y := 0; y < w.Height; y++ {
		dataOfs := y * byteWidth
		rawOfs := y*(byteWidth+1) + 1

		// choose best filter by sum of absolute values
		bestFilter := uint8(0)
		bestSum := int(^uint(0) >> 1) // max int
		for f := 0; f <= 4; f++ {
			sum := filterSum(byte(w.bytesPerPixel), byteWidth, y, f, w.data)
			if sum < bestSum {
				bestSum = sum
				bestFilter = uint8(f)
			}
		}
		raw[rawOfs-1] = bestFilter
		applyFilter(byte(w.bytesPerPixel), bestFilter, w.data[dataOfs:dataOfs+byteWidth], raw[rawOfs:rawOfs+byteWidth], raw[rawOfs-byteWidth-1:rawOfs-1])
	}
	return raw
}

func (w *PNGWriter) Buffer() []byte {
	raw := w.getFilteredRaw()

	// deflate raw
	var deflated bytes.Buffer
	zw := zlib.NewWriter(&deflated)
	_, _ = zw.Write(raw)
	_ = zw.Close()

	// build PNG
	var out bytes.Buffer
	// Signature
	out.Write([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})

	// IHDR chunk
	{
		var ihdr bytes.Buffer
		// Chunk type
		ihdr.Write([]byte{'I', 'H', 'D', 'R'})
		// Data
		var data bytes.Buffer
		binary.Write(&data, binary.BigEndian, uint32(w.Width))
		binary.Write(&data, binary.BigEndian, uint32(w.Height))
		data.WriteByte(w.bitDepth)
		data.WriteByte(w.colorType)
		data.WriteByte(0) // compression
		data.WriteByte(0) // filter
		data.WriteByte(0) // interlace

		// Length
		binary.Write(&out, binary.BigEndian, uint32(data.Len()))
		// Type + Data
		ihdr.Write(data.Bytes())
		out.Write(ihdr.Bytes())
		// CRC
		crc := crc32.ChecksumIEEE(ihdr.Bytes())
		binary.Write(&out, binary.BigEndian, crc)
	}

	// IDAT chunk
	{
		var idat bytes.Buffer
		idat.Write([]byte{'I', 'D', 'A', 'T'})
		// Length
		binary.Write(&out, binary.BigEndian, uint32(deflated.Len()))
		idat.Write(deflated.Bytes())
		out.Write(idat.Bytes())
		crc := crc32.ChecksumIEEE(idat.Bytes())
		binary.Write(&out, binary.BigEndian, crc)
	}

	// IEND chunk
	{
		binary.Write(&out, binary.BigEndian, uint32(0))
		var iend bytes.Buffer
		iend.Write([]byte{'I', 'E', 'N', 'D'})
		out.Write(iend.Bytes())
		crc := crc32.ChecksumIEEE(iend.Bytes())
		binary.Write(&out, binary.BigEndian, crc)
	}

	return out.Bytes()
}

// Filtering implementation mirrors wow.export behavior
func filterSum(bytesPerPixel byte, byteWidth int, row int, filter int, data []byte) int {
	ofs := row * byteWidth
	sum := 0
	switch filter {
	case 0: // None
		for i := 0; i < byteWidth; i++ {
			sum += absI(int(data[ofs+i]))
		}
	case 1: // Sub
		bpp := int(bytesPerPixel)
		for x := 0; x < byteWidth; x++ {
			var left byte
			if x >= bpp {
				left = data[ofs+x-bpp]
			}
			v := int(int8(data[ofs+x] - left))
			if v < 0 {
				v = -v
			}
			sum += v
		}
	case 2: // Up
		prev := ofs - byteWidth
		for x := 0; x < byteWidth; x++ {
			var up byte
			if prev >= 0 {
				up = data[prev+x]
			}
			v := int(int8(data[ofs+x] - up))
			if v < 0 {
				v = -v
			}
			sum += v
		}
	case 3: // Average
		bpp := int(bytesPerPixel)
		prev := ofs - byteWidth
		for x := 0; x < byteWidth; x++ {
			var left byte
			if x >= bpp {
				left = data[ofs+x-bpp]
			}
			var up byte
			if prev >= 0 {
				up = data[prev+x]
			}
			v := int(int8(data[ofs+x] - byte((int(left)+int(up))>>1)))
			if v < 0 {
				v = -v
			}
			sum += v
		}
	case 4: // Paeth
		bpp := int(bytesPerPixel)
		prev := ofs - byteWidth
		for x := 0; x < byteWidth; x++ {
			var left, up, upLeft byte
			if x >= bpp {
				left = data[ofs+x-bpp]
			}
			if prev >= 0 {
				up = data[prev+x]
				if x >= bpp {
					upLeft = data[prev+x-bpp]
				}
			}
			p := paeth(left, up, upLeft)
			v := int(int8(data[ofs+x] - p))
			if v < 0 {
				v = -v
			}
			sum += v
		}
	}
	return sum
}

func applyFilter(bytesPerPixel byte, filter uint8, src []byte, dst []byte, prevRow []byte) {
	byteWidth := len(src)
	switch filter {
	case 0: // None
		copy(dst, src)
	case 1: // Sub
		bpp := int(bytesPerPixel)
		for x := 0; x < byteWidth; x++ {
			var left byte
			if x >= bpp {
				left = src[x-bpp]
			}
			dst[x] = src[x] - left
		}
	case 2: // Up
		for x := 0; x < byteWidth; x++ {
			var up byte
			if len(prevRow) >= byteWidth {
				up = prevRow[x]
			}
			dst[x] = src[x] - up
		}
	case 3: // Average
		bpp := int(bytesPerPixel)
		for x := 0; x < byteWidth; x++ {
			var left byte
			if x >= bpp {
				left = src[x-bpp]
			}
			var up byte
			if len(prevRow) >= byteWidth {
				up = prevRow[x]
			}
			dst[x] = src[x] - byte((int(left)+int(up))>>1)
		}
	case 4: // Paeth
		bpp := int(bytesPerPixel)
		for x := 0; x < byteWidth; x++ {
			var left, up, upLeft byte
			if x >= bpp {
				left = src[x-bpp]
				if len(prevRow) >= byteWidth {
					upLeft = prevRow[x-bpp]
				}
			}
			if len(prevRow) >= byteWidth {
				up = prevRow[x]
			}
			dst[x] = src[x] - paeth(left, up, upLeft)
		}
	}
}

func paeth(left, up, upLeft byte) byte {
	p := int(left) + int(up) - int(upLeft)
	pLeft := absI(p - int(left))
	pUp := absI(p - int(up))
	pUpLeft := absI(p - int(upLeft))
	if pLeft <= pUp && pLeft <= pUpLeft {
		return left
	}
	if pUp <= pUpLeft {
		return up
	}
	return upLeft
}

func absI(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
