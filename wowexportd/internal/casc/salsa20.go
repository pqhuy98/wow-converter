package casc

// Minimal Salsa20 implementation sufficient for BLTE decryption parity.
// Ported from wow.export/src/js/casc/salsa20.js

type salsa20 struct {
	rounds     int
	sigma      [4]uint32
	keyWords   [8]uint32
	nonceWords [2]uint32
	counter    [2]uint32
	block      [64]byte
	blockUsed  int
}

func newSalsa20(nonce [8]byte, key []byte, rounds int) *salsa20 {
	if rounds == 0 {
		rounds = 20
	}
	s := &salsa20{rounds: rounds}
	// Choose constants based on key length (16->SIGMA_32, 32->SIGMA_16 in JS naming)
	if len(key) == 32 {
		s.sigma = [4]uint32{0x61707865, 0x3120646e, 0x79622d36, 0x6b206574}
	} else {
		// Expand 16-byte key to 32
		if len(key) != 16 {
			panic("invalid salsa20 key length")
		}
		k := make([]byte, 32)
		copy(k, key)
		copy(k[16:], key)
		key = k
		s.sigma = [4]uint32{0x61707865, 0x3320646e, 0x79622d32, 0x6b206574}
	}
	for i, j := 0, 0; i < 8; i, j = i+1, j+4 {
		s.keyWords[i] = uint32(key[j]) | uint32(key[j+1])<<8 | uint32(key[j+2])<<16 | uint32(key[j+3])<<24
	}
	s.nonceWords[0] = uint32(nonce[0]) | uint32(nonce[1])<<8 | uint32(nonce[2])<<16 | uint32(nonce[3])<<24
	s.nonceWords[1] = uint32(nonce[4]) | uint32(nonce[5])<<8 | uint32(nonce[6])<<16 | uint32(nonce[7])<<24
	s.blockUsed = 64
	return s
}

func (s *salsa20) reset() { s.counter[0], s.counter[1], s.blockUsed = 0, 0, 64 }

func (s *salsa20) increment() {
	s.counter[0]++
	if s.counter[0] == 0 {
		s.counter[1]++
	}
}

func rotl32(x uint32, n uint) uint32 { return (x<<n | x>>(32-n)) }

func (s *salsa20) generateBlock() {
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

	for i := 0; i < s.rounds; i += 2 {
		x4 ^= rotl32(x0+x12, 7)
		x8 ^= rotl32(x4+x0, 9)
		x12 ^= rotl32(x8+x4, 13)
		x0 ^= rotl32(x12+x8, 18)

		x9 ^= rotl32(x5+x1, 7)
		x13 ^= rotl32(x9+x5, 9)
		x1 ^= rotl32(x13+x9, 13)
		x5 ^= rotl32(x1+x13, 18)

		x14 ^= rotl32(x10+x6, 7)
		x2 ^= rotl32(x14+x10, 9)
		x6 ^= rotl32(x2+x14, 13)
		x10 ^= rotl32(x6+x2, 18)

		x3 ^= rotl32(x15+x11, 7)
		x7 ^= rotl32(x3+x15, 9)
		x11 ^= rotl32(x7+x3, 13)
		x15 ^= rotl32(x11+x7, 18)

		x1 ^= rotl32(x0+x3, 7)
		x2 ^= rotl32(x1+x0, 9)
		x3 ^= rotl32(x2+x1, 13)
		x0 ^= rotl32(x3+x2, 18)

		x6 ^= rotl32(x5+x4, 7)
		x7 ^= rotl32(x6+x5, 9)
		x4 ^= rotl32(x7+x6, 13)
		x5 ^= rotl32(x4+x7, 18)

		x11 ^= rotl32(x10+x9, 7)
		x8 ^= rotl32(x11+x10, 9)
		x9 ^= rotl32(x8+x11, 13)
		x10 ^= rotl32(x9+x8, 18)

		x12 ^= rotl32(x15+x14, 7)
		x13 ^= rotl32(x12+x15, 9)
		x14 ^= rotl32(x13+x12, 13)
		x15 ^= rotl32(x14+x13, 18)
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

	// store little endian
	out := (*[16]uint32)(nil)
	tmp := [16]uint32{x0, x1, x2, x3, x4, x5, x6, x7, x8, x9, x10, x11, x12, x13, x14, x15}
	out = &tmp
	for i := 0; i < 16; i++ {
		v := out[i]
		s.block[i*4+0] = byte(v)
		s.block[i*4+1] = byte(v >> 8)
		s.block[i*4+2] = byte(v >> 16)
		s.block[i*4+3] = byte(v >> 24)
	}
}

func (s *salsa20) getBytes(n int) []byte {
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		if s.blockUsed == 64 {
			s.generateBlock()
			s.increment()
			s.blockUsed = 0
		}
		out[i] = s.block[s.blockUsed]
		s.blockUsed++
	}
	return out
}

func (s *salsa20) process(buf []byte) []byte {
	out := make([]byte, len(buf))
	ks := s.getBytes(len(buf))
	for i := range buf {
		out[i] = ks[i] ^ buf[i]
	}
	return out
}
