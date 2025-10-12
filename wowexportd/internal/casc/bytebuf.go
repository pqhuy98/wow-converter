package casc

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
)

type ByteBuf struct {
	data   []byte
	offset int
}

func NewByteBuf(b []byte) *ByteBuf {
	return &ByteBuf{data: b, offset: 0}
}

func (b *ByteBuf) ByteLength() int { return len(b.data) }
func (b *ByteBuf) Remaining() int  { return len(b.data) - b.offset }
func (b *ByteBuf) Seek(pos int)    { b.offset = pos }
func (b *ByteBuf) Offset() int     { return b.offset }

func (b *ByteBuf) ReadUInt8() uint8 {
	v := b.data[b.offset]
	b.offset++
	return v
}

func (b *ByteBuf) ReadUInt32LE() uint32 {
	d := b.data[b.offset : b.offset+4]
	b.offset += 4
	return uint32(d[0]) | uint32(d[1])<<8 | uint32(d[2])<<16 | uint32(d[3])<<24
}

func (b *ByteBuf) ReadInt32LE() int32 {
	d := b.data[b.offset : b.offset+4]
	b.offset += 4
	return int32(uint32(d[0]) | uint32(d[1])<<8 | uint32(d[2])<<16 | uint32(d[3])<<24)
}

func (b *ByteBuf) ReadUInt16LE() uint16 {
	d := b.data[b.offset : b.offset+2]
	b.offset += 2
	return uint16(d[0]) | uint16(d[1])<<8
}

func (b *ByteBuf) Move(n int) { b.offset += n }

func (b *ByteBuf) ReadInt32BE() int32 {
	d := b.data[b.offset : b.offset+4]
	b.offset += 4
	return int32(uint32(d[0])<<24 | uint32(d[1])<<16 | uint32(d[2])<<8 | uint32(d[3]))
}

func (b *ByteBuf) ReadUInt32BE() uint32 {
	d := b.data[b.offset : b.offset+4]
	b.offset += 4
	return uint32(d[0])<<24 | uint32(d[1])<<16 | uint32(d[2])<<8 | uint32(d[3])
}

func (b *ByteBuf) ReadInt16BE() int {
	d := b.data[b.offset : b.offset+2]
	b.offset += 2
	return int(uint32(d[0])<<8 | uint32(d[1]))
}

// ReadInt40BE reads 5 bytes big-endian integer into int64.
func (b *ByteBuf) ReadInt40BE() int64 {
	d := b.data[b.offset : b.offset+5]
	b.offset += 5
	return int64(d[0])<<32 | int64(d[1])<<24 | int64(d[2])<<16 | int64(d[3])<<8 | int64(d[4])
}

func (b *ByteBuf) ReadHexString(n int) string {
	d := b.data[b.offset : b.offset+n]
	b.offset += n
	out := make([]byte, hex.EncodedLen(len(d)))
	hex.Encode(out, d)
	return string(out)
}

func (b *ByteBuf) ReadBuffer(n int) *ByteBuf {
	d := b.data[b.offset : b.offset+n]
	b.offset += n
	dup := make([]byte, len(d))
	copy(dup, d)
	return NewByteBuf(dup)
}

func (b *ByteBuf) CalculateMD5HexOfRange(start, end int) (string, error) {
	if start < 0 || end > len(b.data) || start > end {
		return "", errors.New("invalid range")
	}
	sum := md5.Sum(b.data[start:end])
	return hex.EncodeToString(sum[:]), nil
}

func (b *ByteBuf) CalculateMD5HexAll() string {
	sum := md5.Sum(b.data)
	return hex.EncodeToString(sum[:])
}
