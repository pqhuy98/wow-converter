// Package casc implements Blizzard CASC archive access.
package casc

// Jenkins96 computes the lookup3-style hash used by CASC helpers.
func Jenkins96(k []byte, init, init2 int32) (uint32, uint32) {
	length := len(k)
	o := 0
	a := uint32(0xDEADBEEF+length+int(init)) & 0xFFFFFFFF
	b := uint32(0xDEADBEEF+length+int(init)) & 0xFFFFFFFF
	c := uint32(0xDEADBEEF+length+int(init)+int(init2)) & 0xFFFFFFFF

	for length > 12 {
		a += uint32(k[o]) | uint32(k[o+1])<<8 | uint32(k[o+2])<<16 | uint32(k[o+3])<<24
		b += uint32(k[o+4]) | uint32(k[o+5])<<8 | uint32(k[o+6])<<16 | uint32(k[o+7])<<24
		c += uint32(k[o+8]) | uint32(k[o+9])<<8 | uint32(k[o+10])<<16 | uint32(k[o+11])<<24

		a -= c
		a ^= rotl32(c, 4) | rotr32(c, 28)
		c = (c + b) & 0xFFFFFFFF
		b -= a
		b ^= rotl32(a, 6) | rotr32(a, 26)
		a = (a + c) & 0xFFFFFFFF
		c -= b
		c ^= rotl32(b, 8) | rotr32(b, 24)
		b = (b + a) & 0xFFFFFFFF
		a -= c
		a ^= rotl32(c, 16) | rotr32(c, 16)
		c = (c + b) & 0xFFFFFFFF
		b -= a
		b ^= rotl32(a, 19) | rotr32(a, 13)
		a = (a + c) & 0xFFFFFFFF
		c -= b
		c ^= rotl32(b, 4) | rotr32(b, 28)
		b = (b + a) & 0xFFFFFFFF

		length -= 12
		o += 12
	}

	if length > 0 {
		switch length {
		case 12:
			c += uint32(k[o+11]) << 24
			fallthrough
		case 11:
			c += uint32(k[o+10]) << 16
			fallthrough
		case 10:
			c += uint32(k[o+9]) << 8
			fallthrough
		case 9:
			c += uint32(k[o+8])
			fallthrough
		case 8:
			b += uint32(k[o+7]) << 24
			fallthrough
		case 7:
			b += uint32(k[o+6]) << 16
			fallthrough
		case 6:
			b += uint32(k[o+5]) << 8
			fallthrough
		case 5:
			b += uint32(k[o+4])
			fallthrough
		case 4:
			a += uint32(k[o+3]) << 24
			fallthrough
		case 3:
			a += uint32(k[o+2]) << 16
			fallthrough
		case 2:
			a += uint32(k[o+1]) << 8
			fallthrough
		case 1:
			a += uint32(k[o])
		}

		c ^= b
		c -= rotl32(b, 14) | rotr32(b, 18)
		a ^= c
		a -= rotl32(c, 11) | rotr32(c, 21)
		b ^= a
		b -= rotl32(a, 25) | rotr32(a, 7)
		c ^= b
		c -= rotl32(b, 16) | rotr32(b, 16)
		a ^= c
		a -= rotl32(c, 4) | rotr32(c, 28)
		b ^= a
		b -= rotl32(a, 14) | rotr32(a, 18)
		c ^= b
		c -= rotl32(b, 24) | rotr32(b, 8)
	}

	return b, c
}

func rotl32(v uint32, n uint) uint32 {
	return (v << n) | (v >> (32 - n))
}

func rotr32(v uint32, n uint) uint32 {
	return (v >> n) | (v << (32 - n))
}
