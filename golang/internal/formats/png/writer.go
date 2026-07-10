// Package png provides a byte-faithful PNG encoder with adaptive filtering.
package png

import (
	"math"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

type filterFn func(data []byte, dataOfs, byteWidth int, raw []byte, rawOfs, bytesPerPixel int)
type filterSumFn func(data []byte, dataOfs, byteWidth, bytesPerPixel int) int

func paeth(left, up, upLeft int) int {
	p := left + up - upLeft
	paethLeft := int(math.Abs(float64(p - left)))
	paethUp := int(math.Abs(float64(p - up)))
	paethUpLeft := int(math.Abs(float64(p - upLeft)))
	if paethLeft <= paethUp && paethLeft <= paethUpLeft {
		return left
	}
	if paethUp <= paethUpLeft {
		return up
	}
	return upLeft
}

var filters = [5]filterFn{
	0: func(data []byte, dataOfs, byteWidth int, raw []byte, rawOfs, _ int) {
		copy(raw[rawOfs:rawOfs+byteWidth], data[dataOfs:dataOfs+byteWidth])
	},
	1: func(data []byte, dataOfs, byteWidth int, raw []byte, rawOfs, bytesPerPixel int) {
		for x := 0; x < byteWidth; x++ {
			left := 0
			if x >= bytesPerPixel {
				left = int(data[dataOfs+x-bytesPerPixel])
			}
			raw[rawOfs+x] = data[dataOfs+x] - byte(left)
		}
	},
	2: func(data []byte, dataOfs, byteWidth int, raw []byte, rawOfs, _ int) {
		for x := 0; x < byteWidth; x++ {
			up := 0
			if dataOfs > 0 {
				up = int(data[dataOfs+x-byteWidth])
			}
			raw[rawOfs+x] = data[dataOfs+x] - byte(up)
		}
	},
	3: func(data []byte, dataOfs, byteWidth int, raw []byte, rawOfs, bytesPerPixel int) {
		for x := 0; x < byteWidth; x++ {
			left := 0
			if x >= bytesPerPixel {
				left = int(data[dataOfs+x-bytesPerPixel])
			}
			up := 0
			if dataOfs > 0 {
				up = int(data[dataOfs+x-byteWidth])
			}
			raw[rawOfs+x] = data[dataOfs+x] - byte((left+up)>>1)
		}
	},
	4: func(data []byte, dataOfs, byteWidth int, raw []byte, rawOfs, bytesPerPixel int) {
		for x := 0; x < byteWidth; x++ {
			left := 0
			if x >= bytesPerPixel {
				left = int(data[dataOfs+x-bytesPerPixel])
			}
			up := 0
			if dataOfs > 0 {
				up = int(data[dataOfs+x-byteWidth])
			}
			upLeft := 0
			if dataOfs > 0 && x >= bytesPerPixel {
				upLeft = int(data[dataOfs+x-(byteWidth+bytesPerPixel)])
			}
			raw[rawOfs+x] = data[dataOfs+x] - byte(paeth(left, up, upLeft))
		}
	},
}

var filterSums = [5]filterSumFn{
	0: func(data []byte, dataOfs, byteWidth, _ int) int {
		sum := 0
		for i := dataOfs; i < dataOfs+byteWidth; i++ {
			sum += int(math.Abs(float64(int8(data[i]))))
		}
		return sum
	},
	1: func(data []byte, dataOfs, byteWidth, bytesPerPixel int) int {
		sum := 0
		for x := 0; x < byteWidth; x++ {
			left := 0
			if x >= bytesPerPixel {
				left = int(data[dataOfs+x-bytesPerPixel])
			}
			sum += int(math.Abs(float64(int(data[dataOfs+x]) - left)))
		}
		return sum
	},
	2: func(data []byte, dataOfs, byteWidth, _ int) int {
		sum := 0
		for x := dataOfs; x < dataOfs+byteWidth; x++ {
			up := 0
			if dataOfs > 0 {
				up = int(data[x-byteWidth])
			}
			sum += int(math.Abs(float64(int(data[x]) - up)))
		}
		return sum
	},
	3: func(data []byte, dataOfs, byteWidth, bytesPerPixel int) int {
		sum := 0
		for x := 0; x < byteWidth; x++ {
			left := 0
			if x >= bytesPerPixel {
				left = int(data[dataOfs+x-bytesPerPixel])
			}
			up := 0
			if dataOfs > 0 {
				up = int(data[dataOfs+x-byteWidth])
			}
			sum += int(math.Abs(float64(int(data[dataOfs+x]) - ((left + up) >> 1))))
		}
		return sum
	},
	4: func(data []byte, dataOfs, byteWidth, bytesPerPixel int) int {
		sum := 0
		for x := 0; x < byteWidth; x++ {
			left := 0
			if x >= bytesPerPixel {
				left = int(data[dataOfs+x-bytesPerPixel])
			}
			up := 0
			if dataOfs > 0 {
				up = int(data[dataOfs+x-byteWidth])
			}
			upLeft := 0
			if dataOfs > 0 && x >= bytesPerPixel {
				upLeft = int(data[dataOfs+x-(byteWidth+bytesPerPixel)])
			}
			sum += int(math.Abs(float64(int(data[dataOfs+x]) - paeth(left, up, upLeft))))
		}
		return sum
	},
}

func filterImage(data []byte, width, height, bytesPerPixel int) []byte {
	byteWidth := width * bytesPerPixel
	dataOfs := 0
	rawOfs := 0
	raw := make([]byte, (byteWidth+1)*height)

	for y := 0; y < height; y++ {
		min := math.MaxInt32
		selectedFilter := 0
		for i := 0; i < len(filters); i++ {
			sum := filterSums[i](data, dataOfs, byteWidth, bytesPerPixel)
			if sum < min {
				selectedFilter = i
				min = sum
			}
		}
		raw[rawOfs] = byte(selectedFilter)
		rawOfs++
		filters[selectedFilter](data, dataOfs, byteWidth, raw, rawOfs, bytesPerPixel)
		rawOfs += byteWidth
		dataOfs += byteWidth
	}
	return raw
}

// Writer encodes RGBA8 PNG images.
type Writer struct {
	Width         int
	Height        int
	BytesPerPixel int
	BitDepth      int
	ColorType     int
	Data          []byte
}

// NewWriter allocates an RGBA PNG writer.
func NewWriter(width, height int) *Writer {
	return &Writer{
		Width:         width,
		Height:        height,
		BytesPerPixel: 4,
		BitDepth:      8,
		ColorType:     6,
		Data:          make([]byte, width*height*4),
	}
}

// PixelData returns the internal pixel buffer.
func (w *Writer) PixelData() []byte { return w.Data }

// Buffer returns the encoded PNG bytes.
func (w *Writer) Buffer() (*buffer.Buffer, error) {
	filtered := filterImage(w.Data, w.Width, w.Height, w.BytesPerPixel)
	filteredBuf := buffer.From(filtered)
	deflated, err := filteredBuf.Deflate()
	if err != nil {
		return nil, err
	}

	out := buffer.Alloc(8+25+deflated.ByteLength()+12+12, false)

	out.WriteUInt32LE(0x474E5089)
	out.WriteUInt32LE(0x0A1A0A0D)

	ihdr := buffer.Alloc(4+13, false)
	ihdr.WriteUInt32LE(0x52444849)
	ihdr.WriteUInt32BE(int64(w.Width))
	ihdr.WriteUInt32BE(int64(w.Height))
	ihdr.WriteUInt8(int64(w.BitDepth))
	ihdr.WriteUInt8(int64(w.ColorType))
	ihdr.WriteUInt8(0)
	ihdr.WriteUInt8(0)
	ihdr.WriteUInt8(0)
	ihdr.Seek(0)

	out.WriteUInt32BE(13)
	out.WriteBuffer(ihdr, 0)
	out.WriteInt32BE(int64(buffer.CRC32(ihdr.Raw())))

	idat := buffer.Alloc(4+deflated.ByteLength(), false)
	idat.WriteUInt32LE(0x54414449)
	idat.WriteBuffer(deflated, 0)
	idat.Seek(0)

	out.WriteUInt32BE(int64(deflated.ByteLength()))
	out.WriteBuffer(idat, 0)
	out.WriteInt32BE(int64(buffer.CRC32(idat.Raw())))

	out.WriteUInt32BE(0)
	out.WriteUInt32LE(0x444E4549)
	out.WriteUInt32LE(0x826042AE)

	return out, nil
}

// WriteToFile writes the PNG to disk.
func (w *Writer) WriteToFile(file string) error {
	buf, err := w.Buffer()
	if err != nil {
		return err
	}
	return buf.WriteToFile(file)
}
