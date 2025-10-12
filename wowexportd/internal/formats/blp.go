package formats

import (
	"encoding/binary"
	"errors"
)

const (
	blpMagic = 0x32504c42 // 'BLP2'
	dxt1Flag = 0x1
	dxt3Flag = 0x2
	dxt5Flag = 0x4
)

type blpImage struct {
	encoding      uint8
	alphaDepth    uint8
	alphaEncoding uint8
	containsMips  uint8
	width         uint32
	height        uint32
	mapOffsets    [16]uint32
	mapSizes      [16]uint32
	mapCount      int
	palette       [256][4]byte
	rawData       []byte
	scaledWidth   uint32
	scaledHeight  uint32
	scaledLength  uint32
}

func NewBLP(raw []byte) (*blpImage, error) {
	if len(raw) < 148 {
		return nil, errors.New("invalid BLP (short)")
	}
	if binary.LittleEndian.Uint32(raw[0:4]) != blpMagic {
		return nil, errors.New("invalid BLP magic")
	}
	if binary.LittleEndian.Uint32(raw[4:8]) != 1 {
		return nil, errors.New("unsupported BLP type")
	}
	img := &blpImage{}
	img.encoding = raw[8]
	img.alphaDepth = raw[9]
	img.alphaEncoding = raw[10]
	img.containsMips = raw[11]
	img.width = binary.LittleEndian.Uint32(raw[12:16])
	img.height = binary.LittleEndian.Uint32(raw[16:20])
	for i := 0; i < 16; i++ {
		img.mapOffsets[i] = binary.LittleEndian.Uint32(raw[20+i*4 : 24+i*4])
	}
	for i := 0; i < 16; i++ {
		img.mapSizes[i] = binary.LittleEndian.Uint32(raw[84+i*4 : 88+i*4])
	}
	// palette for encoding=1
	if img.encoding == 1 {
		// each palette entry is BGRA 4 bytes starting at 148
		base := 148
		for i := 0; i < 256; i++ {
			b := raw[base+i*4+0]
			g := raw[base+i*4+1]
			r := raw[base+i*4+2]
			a := byte(255)
			img.palette[i] = [4]byte{r, g, b, a}
		}
	}
	// default to mip 0
	if err := img.prepare(raw, 0); err != nil {
		return nil, err
	}
	return img, nil
}

func (b *blpImage) prepare(raw []byte, mip int) error {
	// count maps
	b.mapCount = 0
	for i := 0; i < 16; i++ {
		if b.mapOffsets[i] != 0 {
			b.mapCount++
		}
	}
	if b.mapCount == 0 {
		return errors.New("invalid BLP (no mipmaps)")
	}
	if mip < 0 || mip >= b.mapCount {
		mip = 0
	}
	scale := uint32(1 << mip)
	b.scaledWidth = b.width / scale
	b.scaledHeight = b.height / scale
	b.scaledLength = b.scaledWidth * b.scaledHeight
	ofs := b.mapOffsets[mip]
	size := b.mapSizes[mip]
	if int(ofs+size) > len(raw) {
		return errors.New("invalid BLP (mipmap range)")
	}
	b.rawData = raw[ofs : ofs+size]
	return nil
}

func (b *blpImage) getAlpha(idx uint32) byte {
	switch b.alphaDepth {
	case 1:
		byteIndex := b.scaledLength + (idx / 8)
		bit := idx % 8
		if b.rawData[byteIndex]&byte(1<<bit) != 0 {
			return 0xFF
		}
		return 0x00
	case 4:
		byteIndex := b.scaledLength + (idx / 2)
		q := b.rawData[byteIndex]
		if idx%2 == 0 {
			return (q & 0x0F) << 4
		}
		return (q & 0xF0)
	case 8:
		return b.rawData[b.scaledLength+idx]
	default:
		return 0xFF
	}
}

func (b *blpImage) writeUncompressed(dst []byte, mask uint8) {
	for i := uint32(0); i < b.scaledLength; i++ {
		paletteIndex := b.rawData[i]
		col := b.palette[paletteIndex]
		ofs := i * 4
		if mask&0b1 != 0 {
			dst[ofs+0] = col[0]
		} else {
			dst[ofs+0] = 0
		}
		if mask&0b10 != 0 {
			dst[ofs+1] = col[1]
		} else {
			dst[ofs+1] = 0
		}
		if mask&0b100 != 0 {
			dst[ofs+2] = col[2]
		} else {
			dst[ofs+2] = 0
		}
		if mask&0b1000 != 0 {
			dst[ofs+3] = b.getAlpha(i)
		} else {
			dst[ofs+3] = 255
		}
	}
}

func (b *blpImage) marshalBGRA(dst []byte, mask uint8) {
	n := uint32(len(b.rawData) / 4)
	for i := uint32(0); i < n; i++ {
		ofs := i * 4
		// raw is BGRA
		if mask&0b1 != 0 {
			dst[ofs+0] = b.rawData[ofs+2]
		} else {
			dst[ofs+0] = 0
		}
		if mask&0b10 != 0 {
			dst[ofs+1] = b.rawData[ofs+1]
		} else {
			dst[ofs+1] = 0
		}
		if mask&0b100 != 0 {
			dst[ofs+2] = b.rawData[ofs+0]
		} else {
			dst[ofs+2] = 0
		}
		if mask&0b1000 != 0 {
			dst[ofs+3] = b.rawData[ofs+3]
		} else {
			dst[ofs+3] = 255
		}
	}
}

func (b *blpImage) writeCompressed(dst []byte, mask uint8) {
	// Determine block type from alpha depth/encoding
	flags := 0
	if b.alphaDepth > 1 {
		if b.alphaEncoding == 7 {
			flags = dxt5Flag
		} else {
			flags = dxt3Flag
		}
	} else {
		flags = dxt1Flag
	}
	sw := int(b.scaledWidth)
	sh := int(b.scaledHeight)
	pos := 0
	blockBytes := 8
	if flags != dxt1Flag {
		blockBytes = 16
	}
	target := make([]byte, 4*16)
	for y := 0; y < sh; y += 4 {
		for x := 0; x < sw; x += 4 {
			blockPos := 0
			if len(b.rawData) == pos {
				continue
			}
			colourIndex := pos
			if flags != dxt1Flag {
				colourIndex += 8
			}
			// decode colors
			a := unpackColour(b.rawData, colourIndex+0)
			c0 := colourToRGBA(a)
			bcol := unpackColour(b.rawData, colourIndex+2)
			c1 := colourToRGBA(bcol)
			colours := make([][4]byte, 4)
			colours[0] = c0
			colours[1] = c1
			isDXT1 := flags == dxt1Flag
			if isDXT1 && a <= bcol {
				colours[2] = avgRGBA(c0, c1)
				colours[3] = [4]byte{0, 0, 0, 0}
			} else {
				colours[2] = lerpRGBA(c0, c1, 2, 3)
				colours[3] = lerpRGBA(c0, c1, 1, 3)
			}
			index := [4]byte{b.rawData[colourIndex+4], b.rawData[colourIndex+5], b.rawData[colourIndex+6], b.rawData[colourIndex+7]}
			for i := 0; i < 4; i++ {
				packed := index[i]
				for j := 0; j < 4; j++ {
					idx := (packed >> (uint(j) * 2)) & 0x3
					ofs := (i*4 + j) * 4
					col := colours[idx]
					target[ofs+0] = col[0]
					target[ofs+1] = col[1]
					target[ofs+2] = col[2]
					target[ofs+3] = col[3]
				}
			}
			if flags == dxt3Flag {
				for i := 0; i < 8; i++ {
					quant := b.rawData[pos+i]
					low := quant & 0x0F
					high := (quant & 0xF0) >> 4
					target[8*i+3] = (low | (low << 4))
					target[8*i+7] = (high | (high << 4))
				}
			} else if flags == dxt5Flag {
				a0 := b.rawData[pos+0]
				a1 := b.rawData[pos+1]
				coloursA := make([]byte, 8)
				coloursA[0] = a0
				coloursA[1] = a1
				if a0 <= a1 {
					for i := 1; i < 5; i++ {
						coloursA[i+1] = byte((((5-i)*int(a0) + i*int(a1)) / 5))
					}
					coloursA[6] = 0
					coloursA[7] = 255
				} else {
					for i := 1; i < 7; i++ {
						coloursA[i+1] = byte((((7-i)*int(a0) + i*int(a1)) / 7))
					}
				}
				blockPos := 2
				indices := make([]byte, 16)
				indicesPos := 0
				for i := 0; i < 2; i++ {
					var value uint32
					for j := 0; j < 3; j++ {
						value |= uint32(b.rawData[pos+blockPos+j]) << (8 * uint32(j))
					}
					blockPos += 3
					for j := 0; j < 8; j++ {
						indices[indicesPos] = byte((value >> (3 * uint32(j))) & 0x07)
						indicesPos++
					}
				}
				for i := 0; i < 16; i++ {
					target[4*i+3] = coloursA[indices[i]]
				}
			}
			for py := 0; py < 4; py++ {
				for px := 0; px < 4; px++ {
					sx := x + px
					sy := y + py
					if sx < sw && sy < sh {
						pixel := 4 * (sw*sy + sx)
						dst[pixel+0] = cond(mask&0b1 != 0, target[blockPos+0], 0)
						dst[pixel+1] = cond(mask&0b10 != 0, target[blockPos+1], 0)
						dst[pixel+2] = cond(mask&0b100 != 0, target[blockPos+2], 0)
						dst[pixel+3] = cond(mask&0b1000 != 0, target[blockPos+3], 255)
					}
					blockPos += 4
				}
			}
			pos += blockBytes
		}
	}
}

func unpackColour(block []byte, ofs int) uint16 {
	return uint16(block[ofs]) | uint16(block[ofs+1])<<8
}

func colourToRGBA(v uint16) [4]byte {
	r := byte(((v >> 11) & 0x1F) << 3)
	g := byte(((v >> 5) & 0x3F) << 2)
	b := byte((v & 0x1F) << 3)
	// add low bits for better spread
	r |= r >> 5
	g |= g >> 6
	b |= b >> 5
	return [4]byte{r, g, b, 255}
}

func avgRGBA(a, b [4]byte) [4]byte {
	return [4]byte{byte((int(a[0]) + int(b[0])) / 2), byte((int(a[1]) + int(b[1])) / 2), byte((int(a[2]) + int(b[2])) / 2), 255}
}

func lerpRGBA(a, b [4]byte, n, d int) [4]byte {
	return [4]byte{byte((n*int(a[0]) + (d-n)*int(b[0])) / d), byte((n*int(a[1]) + (d-n)*int(b[1])) / d), byte((n*int(a[2]) + (d-n)*int(b[2])) / d), 255}
}

func cond(ok bool, a, b byte) byte {
	if ok {
		return a
	}
	return b
}
