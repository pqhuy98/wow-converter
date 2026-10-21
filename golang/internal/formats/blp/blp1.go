package blp

import (
	"encoding/binary"
	"fmt"
	"image"
)

// DecodePalettedBLP1 decodes a mip of the WC3 paletted format written by this
// package, including compact0/1/4/8-bit alpha. WoW BLP2 uses NewBLPImage.
func DecodePalettedBLP1(data []byte, level int) (*image.NRGBA, error) {
	if len(data) < 1180 || string(data[:4]) != "BLP1" || binary.LittleEndian.Uint32(data[4:]) != 1 || level < 0 || level >= 16 {
		return nil, fmt.Errorf("invalid paletted BLP1")
	}
	bits := binary.LittleEndian.Uint32(data[8:])
	if bits != 0 && bits != 1 && bits != 4 && bits != 8 {
		return nil, fmt.Errorf("unsupported BLP1 alpha depth %d", bits)
	}
	width, height := uint64(binary.LittleEndian.Uint32(data[12:])), uint64(binary.LittleEndian.Uint32(data[16:]))
	if width == 0 || height == 0 || width > 16384 || height > 16384 {
		return nil, fmt.Errorf("invalid BLP1 dimensions")
	}
	w, h := max(uint64(1), width>>level), max(uint64(1), height>>level)
	pixels := w * h
	offset, size := uint64(binary.LittleEndian.Uint32(data[28+level*4:])), uint64(binary.LittleEndian.Uint32(data[92+level*4:]))
	required := pixels + (pixels*uint64(bits)+7)/8
	if offset < 1180 || size < required || offset+size > uint64(len(data)) {
		return nil, fmt.Errorf("invalid BLP1 mip %d", level)
	}
	img := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	for i := 0; i < int(pixels); i++ {
		palette := 156 + int(data[int(offset)+i])*4
		at := i * 4
		img.Pix[at], img.Pix[at+1], img.Pix[at+2] = data[palette+2], data[palette+1], data[palette]
		a := byte(255)
		alpha := int(offset + pixels)
		switch bits {
		case 1:
			a = ((data[alpha+i/8] >> uint(i%8)) & 1) * 255
		case 4:
			a = ((data[alpha+i/2] >> uint((i%2)*4)) & 15) * 17
		case 8:
			a = data[alpha+i]
		}
		img.Pix[at+3] = a
	}
	return img, nil
}
