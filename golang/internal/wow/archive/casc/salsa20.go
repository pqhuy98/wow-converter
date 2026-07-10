package casc

import (
	"encoding/hex"
	"fmt"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

var sigma32 = []uint32{0x61707865, 0x3320646E, 0x79622D32, 0x6B206574}
var sigma16 = []uint32{0x61707865, 0x3120646E, 0x79622D36, 0x6B206574}

// Salsa20 implements the stream cipher used by CASC data helpers.
type Salsa20 struct {
	rounds     int
	sigma      []uint32
	keyWords   [8]uint32
	nonceWords [2]uint32
	counter    [2]uint32
	block      [64]byte
	blockUsed  int
}

// NewSalsa20 creates a Salsa20 instance.
func NewSalsa20(nonce []byte, keyHex string, rounds int) (*Salsa20, error) {
	if len(nonce) != 8 {
		return nil, fmt.Errorf("unexpected nonce length. 8 bytes expected, got %d", len(nonce))
	}
	if len(keyHex) != 32 && len(keyHex) != 64 {
		return nil, fmt.Errorf("unexpected key length. 16 or 32 bytes expected, got %d", len(keyHex))
	}
	if rounds == 0 {
		rounds = 20
	}
	s := &Salsa20{rounds: rounds, blockUsed: 64}
	if len(keyHex) == 32 {
		s.sigma = sigma16
	} else {
		s.sigma = sigma32
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, err
	}
	s.SetKey(key)
	s.SetNonce(nonce)
	return s, nil
}

// SetKey sets the encryption key.
func (s *Salsa20) SetKey(key []byte) {
	if len(key) == 16 {
		expanded := make([]byte, 32)
		copy(expanded, key)
		copy(expanded[16:], key)
		key = expanded
	}
	for i, j := 0, 0; i < 8; i, j = i+1, j+4 {
		s.keyWords[i] = uint32(key[j]) | uint32(key[j+1])<<8 | uint32(key[j+2])<<16 | uint32(key[j+3])<<24
	}
	s.reset()
}

// SetNonce sets the nonce.
func (s *Salsa20) SetNonce(nonce []byte) {
	s.nonceWords[0] = uint32(nonce[0]) | uint32(nonce[1])<<8 | uint32(nonce[2])<<16 | uint32(nonce[3])<<24
	s.nonceWords[1] = uint32(nonce[4]) | uint32(nonce[5])<<8 | uint32(nonce[6])<<16 | uint32(nonce[7])<<24
	s.reset()
}

// GetBytes returns byteCount keystream bytes.
func (s *Salsa20) GetBytes(byteCount int) *buffer.Buffer {
	out := buffer.Alloc(byteCount, false)
	for i := 0; i < byteCount; i++ {
		if s.blockUsed == 64 {
			s.generateBlock()
			s.increment()
			s.blockUsed = 0
		}
		out.WriteUInt8(int64(s.block[s.blockUsed]))
		s.blockUsed++
	}
	out.Seek(0)
	return out
}

// Process XORs the buffer with the keystream.
func (s *Salsa20) Process(buf *buffer.Buffer) *buffer.Buffer {
	out := buffer.Alloc(buf.ByteLength(), false)
	bytes := s.GetBytes(buf.ByteLength())
	buf.Seek(0)
	for i := 0; i < buf.ByteLength(); i++ {
		out.WriteUInt8(bytes.ReadUInt8().(int64) ^ buf.ReadUInt8().(int64))
	}
	out.Seek(0)
	return out
}

func (s *Salsa20) reset() {
	s.counter = [2]uint32{0, 0}
	s.blockUsed = 64
}

func (s *Salsa20) increment() {
	s.counter[0] = (s.counter[0] + 1) & 0xFFFFFFFF
	if s.counter[0] == 0 {
		s.counter[1] = (s.counter[1] + 1) & 0xFFFFFFFF
	}
}

func (s *Salsa20) generateBlock() {
	j0 := s.sigma[0]
	j1 := s.keyWords[0]
	j2 := s.keyWords[1]
	j3 := s.keyWords[2]
	j4 := s.keyWords[3]
	j5 := s.sigma[1]
	j6 := s.nonceWords[0]
	j7 := s.nonceWords[1]
	j8 := s.counter[0]
	j9 := s.counter[1]
	j10 := s.sigma[2]
	j11 := s.keyWords[4]
	j12 := s.keyWords[5]
	j13 := s.keyWords[6]
	j14 := s.keyWords[7]
	j15 := s.sigma[3]

	x0, x1, x2, x3 := j0, j1, j2, j3
	x4, x5, x6, x7 := j4, j5, j6, j7
	x8, x9, x10, x11 := j8, j9, j10, j11
	x12, x13, x14, x15 := j12, j13, j14, j15

	var u uint32
	for i := 0; i < s.rounds; i += 2 {
		u = x0 + x12
		x4 ^= rotl32(u, 7)
		u = x4 + x0
		x8 ^= rotl32(u, 9)
		u = x8 + x4
		x12 ^= rotl32(u, 13)
		u = x12 + x8
		x0 ^= rotl32(u, 18)

		u = x5 + x1
		x9 ^= rotl32(u, 7)
		u = x9 + x5
		x13 ^= rotl32(u, 9)
		u = x13 + x9
		x1 ^= rotl32(u, 13)
		u = x1 + x13
		x5 ^= rotl32(u, 18)

		u = x10 + x6
		x14 ^= rotl32(u, 7)
		u = x14 + x10
		x2 ^= rotl32(u, 9)
		u = x2 + x14
		x6 ^= rotl32(u, 13)
		u = x6 + x2
		x10 ^= rotl32(u, 18)

		u = x15 + x11
		x3 ^= rotl32(u, 7)
		u = x3 + x15
		x7 ^= rotl32(u, 9)
		u = x7 + x3
		x11 ^= rotl32(u, 13)
		u = x11 + x7
		x15 ^= rotl32(u, 18)

		u = x0 + x3
		x1 ^= rotl32(u, 7)
		u = x1 + x0
		x2 ^= rotl32(u, 9)
		u = x2 + x1
		x3 ^= rotl32(u, 13)
		u = x3 + x2
		x0 ^= rotl32(u, 18)

		u = x5 + x4
		x6 ^= rotl32(u, 7)
		u = x6 + x5
		x7 ^= rotl32(u, 9)
		u = x7 + x6
		x4 ^= rotl32(u, 13)
		u = x4 + x7
		x5 ^= rotl32(u, 18)

		u = x10 + x9
		x11 ^= rotl32(u, 7)
		u = x11 + x10
		x8 ^= rotl32(u, 9)
		u = x8 + x11
		x9 ^= rotl32(u, 13)
		u = x9 + x8
		x10 ^= rotl32(u, 18)

		u = x15 + x14
		x12 ^= rotl32(u, 7)
		u = x12 + x15
		x13 ^= rotl32(u, 9)
		u = x13 + x12
		x14 ^= rotl32(u, 13)
		u = x14 + x13
		x15 ^= rotl32(u, 18)
	}

	x0 += j0
	x1 += j1
	x2 += j2
	x3 += j3
	x4 += j4
	x5 += j5
	x6 += j6
	x7 += j7
	x8 += j8
	x9 += j9
	x10 += j10
	x11 += j11
	x12 += j12
	x13 += j13
	x14 += j14
	x15 += j15

	words := []uint32{x0, x1, x2, x3, x4, x5, x6, x7, x8, x9, x10, x11, x12, x13, x14, x15}
	for w := 0; w < 16; w++ {
		x := words[w]
		s.block[w*4+0] = byte(x & 0xFF)
		s.block[w*4+1] = byte((x >> 8) & 0xFF)
		s.block[w*4+2] = byte((x >> 16) & 0xFF)
		s.block[w*4+3] = byte((x >> 24) & 0xFF)
	}
}
