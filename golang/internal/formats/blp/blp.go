// Package blp implements BLP2 texture decoding and PNG export.
package blp

import (
	"fmt"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/formats/png"
)

const (
	dxt1 = 0x1
	dxt3 = 0x2
	dxt5 = 0x4

	blpMagic = 0x32504C42
)

// BLPImage decodes WoW BLP2 textures.
type BLPImage struct {
	data            *buffer.Buffer
	Encoding        uint8
	AlphaDepth      uint8
	AlphaEncoding   uint8
	ContainsMipmaps uint8
	Width           uint32
	Height          uint32
	MapOffsets      []uint32
	MapSizes        []uint32
	MapCount        int
	Palette         [256][4]uint8

	scale          int
	ScaledWidth    int
	ScaledHeight   int
	scaledLength   int
	rawData        []byte
}

// NewBLPImage parses a BLP2 file.
func NewBLPImage(data *buffer.Buffer) (*BLPImage, error) {
	img := &BLPImage{data: data}
	if uint32(data.ReadUInt32LE().(int64)) != blpMagic {
		return nil, fmt.Errorf("provided data is not a BLP file (invalid header magic)")
	}
	if uint32(data.ReadUInt32LE().(int64)) != 1 {
		return nil, fmt.Errorf("unsupported BLP type")
	}
	img.Encoding = uint8(data.ReadUInt8().(int64))
	img.AlphaDepth = uint8(data.ReadUInt8().(int64))
	img.AlphaEncoding = uint8(data.ReadUInt8().(int64))
	img.ContainsMipmaps = uint8(data.ReadUInt8().(int64))
	img.Width = uint32(data.ReadUInt32LE().(int64))
	img.Height = uint32(data.ReadUInt32LE().(int64))
	img.MapOffsets = readU32Slice(data, 16)
	img.MapSizes = readU32Slice(data, 16)
	for _, ofs := range img.MapOffsets {
		if ofs != 0 {
			img.MapCount++
		}
	}
	if img.Encoding == 1 {
		for i := 0; i < 256; i++ {
			raw := data.ReadUInt8(4).([]int64)
			for j := 0; j < 4; j++ {
				img.Palette[i][j] = uint8(raw[j])
			}
		}
	}
	return img, nil
}

// ToPNG encodes the requested mipmap level as PNG bytes.
func (img *BLPImage) ToPNG(mask byte, mipmap int) (*buffer.Buffer, error) {
	img.prepare(mipmap)

	pngWriter := png.NewWriter(img.ScaledWidth, img.ScaledHeight)
	pixelData := pngWriter.PixelData()

	switch img.Encoding {
	case 1:
		img.getUncompressed(pixelData, mask)
	case 2:
		img.getCompressed(pixelData, mask)
	case 3:
		img.marshalBGRA(pixelData, mask)
	default:
		return nil, fmt.Errorf("unsupported BLP encoding: %d", img.Encoding)
	}

	return pngWriter.Buffer()
}

// SaveToPNG writes the decoded mipmap as a PNG file.
func (img *BLPImage) SaveToPNG(file string, mask byte, mipmap int) error {
	buf, err := img.ToPNG(mask, mipmap)
	if err != nil {
		return err
	}
	return buf.WriteToFile(file)
}

// ToBuffer returns RGBA bytes for the requested mipmap.
func (img *BLPImage) ToBuffer(mipmap int, mask byte) (*buffer.Buffer, error) {
	img.prepare(mipmap)

	switch img.Encoding {
	case 1:
		return img.getUncompressed(nil, mask), nil
	case 2:
		return img.getCompressed(nil, mask), nil
	case 3:
		return img.marshalBGRA(nil, mask), nil
	default:
		return nil, fmt.Errorf("unsupported BLP encoding: %d", img.Encoding)
	}
}

// GetRawMipmap returns raw mipmap payload bytes.
func (img *BLPImage) GetRawMipmap(mipmap int) []byte {
	img.prepare(mipmap)
	out := make([]byte, len(img.rawData))
	copy(out, img.rawData)
	return out
}

// ToUInt8Array returns RGBA bytes for the requested mipmap.
func (img *BLPImage) ToUInt8Array(mipmap int, mask byte) ([]byte, error) {
	img.prepare(mipmap)

	arr := make([]byte, img.ScaledWidth*img.ScaledHeight*4)
	switch img.Encoding {
	case 1:
		img.getUncompressed(arr, mask)
	case 2:
		img.getCompressed(arr, mask)
	case 3:
		img.marshalBGRA(arr, mask)
	default:
		return nil, fmt.Errorf("unsupported BLP encoding: %d", img.Encoding)
	}
	return arr, nil
}

func (img *BLPImage) prepare(mipmap int) {
	level := mipmap
	if level < 0 {
		level = 0
	}
	if level >= img.MapCount {
		level = img.MapCount - 1
	}
	img.scale = 1 << level
	img.ScaledWidth = int(img.Width) / img.scale
	img.ScaledHeight = int(img.Height) / img.scale
	img.scaledLength = img.ScaledWidth * img.ScaledHeight

	img.data.Seek(int(img.MapOffsets[level]))
	img.rawData = img.data.ReadBuffer(buffer.ReadBufferOptions{
		Length: int(img.MapSizes[level]),
		Wrap:   false,
	}).([]byte)
}

func (img *BLPImage) getAlpha(index int) uint8 {
	switch img.AlphaDepth {
	case 1:
		byteVal := img.rawData[img.scaledLength+index/8]
		if (byteVal & (0x01 << (index % 8))) == 0 {
			return 0x00
		}
		return 0xFF
	case 4:
		byteVal := img.rawData[img.scaledLength+index/2]
		if index%2 == 0 {
			return (byteVal & 0x0F) << 4
		}
		return byteVal & 0xF0
	case 8:
		return img.rawData[img.scaledLength+index]
	default:
		return 0xFF
	}
}

func unpackColour(block []byte, index, ofs int, colour []byte, colourOfs int) int {
	value := int(block[index+ofs]) | (int(block[index+1+ofs]) << 8)

	r := (value >> 11) & 0x1F
	g := (value >> 5) & 0x3F
	b := value & 0x1F

	colour[colourOfs] = byte((r << 3) | (r >> 2))
	colour[colourOfs+1] = byte((g << 2) | (g >> 4))
	colour[colourOfs+2] = byte((b << 3) | (b >> 2))
	colour[colourOfs+3] = 255

	return value
}

func (img *BLPImage) getCompressed(canvasData []byte, mask byte) *buffer.Buffer {
	flags := dxt1
	if img.AlphaDepth > 1 {
		if img.AlphaEncoding == 7 {
			flags = dxt5
		} else {
			flags = dxt3
		}
	}

	var data []byte
	if canvasData == nil {
		data = make([]byte, img.ScaledWidth*img.ScaledHeight*4)
	} else {
		data = canvasData
	}

	pos := 0
	blockBytes := 8
	if flags&dxt1 == 0 {
		blockBytes = 16
	}
	target := make([]byte, 4*16)

	sw := img.ScaledWidth
	sh := img.ScaledHeight

	for y := 0; y < sh; y += 4 {
		for x := 0; x < sw; x += 4 {
			blockPos := 0

			if len(img.rawData) == pos {
				continue
			}

			colourIndex := pos
			if flags&(dxt3|dxt5) != 0 {
				colourIndex += 8
			}

			isDXT1 := flags&dxt1 != 0
			colours := make([]byte, 16*4)
			a := unpackColour(img.rawData, colourIndex, 0, colours, 0)
			b := unpackColour(img.rawData, colourIndex, 2, colours, 4)

			for i := 0; i < 3; i++ {
				c := int(colours[i])
				d := int(colours[i+4])
				if isDXT1 && a <= b {
					colours[i+8] = byte((c + d) / 2)
					colours[i+12] = 0
				} else {
					colours[i+8] = byte((2*c + d) / 3)
					colours[i+12] = byte((c + 2*d) / 3)
				}
			}
			colours[8+3] = 255
			if isDXT1 && a <= b {
				colours[12+3] = 0
			} else {
				colours[12+3] = 255
			}

			index := make([]int, 16)
			for i := 0; i < 4; i++ {
				packed := int(img.rawData[colourIndex+4+i])
				index[i*4] = packed & 0x3
				index[1+i*4] = (packed >> 2) & 0x3
				index[2+i*4] = (packed >> 4) & 0x3
				index[3+i*4] = (packed >> 6) & 0x3
			}

			for i := 0; i < 16; i++ {
				ofs := index[i] * 4
				target[4*i] = colours[ofs]
				target[4*i+1] = colours[ofs+1]
				target[4*i+2] = colours[ofs+2]
				target[4*i+3] = colours[ofs+3]
			}

			if flags&dxt3 != 0 {
				for i := 0; i < 8; i++ {
					quant := img.rawData[pos+i]
					low := quant & 0x0F
					high := quant & 0xF0
					target[8*i+3] = low | (low << 4)
					target[8*i+7] = high | (high >> 4)
				}
			} else if flags&dxt5 != 0 {
				a0 := int(img.rawData[pos])
				a1 := int(img.rawData[pos+1])

				alphaColours := make([]int, 8)
				alphaColours[0] = a0
				alphaColours[1] = a1

				if a0 <= a1 {
					for i := 1; i < 5; i++ {
						alphaColours[i+1] = ((5-i)*a0 + i*a1) / 5
					}
					alphaColours[6] = 0
					alphaColours[7] = 255
				} else {
					for i := 1; i < 7; i++ {
						alphaColours[i+1] = ((7-i)*a0 + i*a1) / 7
					}
				}

				indices := make([]int, 16)
				alphaBlockPos := 2
				indicesPos := 0
				for i := 0; i < 2; i++ {
					value := 0
					for j := 0; j < 3; j++ {
						byteVal := int(img.rawData[pos+alphaBlockPos])
						alphaBlockPos++
						value |= byteVal << (8 * j)
					}
					for j := 0; j < 8; j++ {
						indices[indicesPos] = (value >> (3 * j)) & 0x07
						indicesPos++
					}
				}

				for i := 0; i < 16; i++ {
					target[4*i+3] = byte(alphaColours[indices[i]])
				}
			}

			for pY := 0; pY < 4; pY++ {
				for pX := 0; pX < 4; pX++ {
					sX := x + pX
					sY := y + pY
					if sX < sw && sY < sh {
						pixel := 4 * (sw*sY + sX)
						if mask&0b1 != 0 {
							data[pixel] = target[blockPos]
						} else {
							data[pixel] = 0
						}
						if mask&0b10 != 0 {
							data[pixel+1] = target[blockPos+1]
						} else {
							data[pixel+1] = 0
						}
						if mask&0b100 != 0 {
							data[pixel+2] = target[blockPos+2]
						} else {
							data[pixel+2] = 0
						}
						if mask&0b1000 != 0 {
							data[pixel+3] = target[blockPos+3]
						} else {
							data[pixel+3] = 255
						}
					}
					blockPos += 4
				}
			}
			pos += blockBytes
		}
	}

	if canvasData == nil {
		buf := buffer.Alloc(len(data), false)
		buf.WriteBuffer(data, len(data))
		buf.Seek(0)
		return buf
	}
	return nil
}

func (img *BLPImage) getUncompressed(canvasData []byte, mask byte) *buffer.Buffer {
	if canvasData != nil {
		for i := 0; i < img.scaledLength; i++ {
			ofs := i * 4
			colour := img.Palette[img.rawData[i]]
			if mask&0b1 != 0 {
				canvasData[ofs] = colour[2]
			} else {
				canvasData[ofs] = 0
			}
			if mask&0b10 != 0 {
				canvasData[ofs+1] = colour[1]
			} else {
				canvasData[ofs+1] = 0
			}
			if mask&0b100 != 0 {
				canvasData[ofs+2] = colour[0]
			} else {
				canvasData[ofs+2] = 0
			}
			if mask&0b1000 != 0 {
				canvasData[ofs+3] = img.getAlpha(i)
			} else {
				canvasData[ofs+3] = 255
			}
		}
		return nil
	}

	buf := buffer.Alloc(img.scaledLength*4, false)
	for i := 0; i < img.scaledLength; i++ {
		colour := img.Palette[img.rawData[i]]
		if mask&0b1 != 0 {
			buf.WriteUInt8(int64(colour[2]))
		} else {
			buf.WriteUInt8(0)
		}
		if mask&0b10 != 0 {
			buf.WriteUInt8(int64(colour[1]))
		} else {
			buf.WriteUInt8(0)
		}
		if mask&0b100 != 0 {
			buf.WriteUInt8(int64(colour[0]))
		} else {
			buf.WriteUInt8(0)
		}
		if mask&0b1000 != 0 {
			buf.WriteUInt8(int64(img.getAlpha(i)))
		} else {
			buf.WriteUInt8(255)
		}
	}
	buf.Seek(0)
	return buf
}

func (img *BLPImage) marshalBGRA(canvasData []byte, mask byte) *buffer.Buffer {
	data := img.rawData
	n := len(data) / 4

	if canvasData != nil {
		for i := 0; i < n; i++ {
			ofs := i * 4
			if mask&0b1 != 0 {
				canvasData[ofs] = data[ofs+2]
			} else {
				canvasData[ofs] = 0
			}
			if mask&0b10 != 0 {
				canvasData[ofs+1] = data[ofs+1]
			} else {
				canvasData[ofs+1] = 0
			}
			if mask&0b100 != 0 {
				canvasData[ofs+2] = data[ofs]
			} else {
				canvasData[ofs+2] = 0
			}
			if mask&0b1000 != 0 {
				canvasData[ofs+3] = data[ofs+3]
			} else {
				canvasData[ofs+3] = 255
			}
		}
		return nil
	}

	buf := buffer.Alloc(len(data), false)
	for i := 0; i < n; i++ {
		ofs := i * 4
		if mask&0b1 != 0 {
			buf.WriteUInt8(int64(data[ofs+2]))
		} else {
			buf.WriteUInt8(0)
		}
		if mask&0b10 != 0 {
			buf.WriteUInt8(int64(data[ofs+1]))
		} else {
			buf.WriteUInt8(0)
		}
		if mask&0b100 != 0 {
			buf.WriteUInt8(int64(data[ofs]))
		} else {
			buf.WriteUInt8(0)
		}
		if mask&0b1000 != 0 {
			buf.WriteUInt8(int64(data[ofs+3]))
		} else {
			buf.WriteUInt8(255)
		}
	}
	buf.Seek(0)
	return buf
}

func readU32Slice(b *buffer.Buffer, count int) []uint32 {
	raw := b.ReadUInt32LE(count).([]int64)
	out := make([]uint32, count)
	for i, v := range raw {
		out[i] = uint32(v)
	}
	return out
}
