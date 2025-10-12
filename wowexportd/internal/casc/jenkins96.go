package casc

// Jenkins96 implements the 96-bit Jenkins hash variant used by wow.export.
// It returns (b, c) 32-bit values (c is commonly used as the 32-bit hash).
// Ported from wow.export/src/js/casc/jenkins96.js
func Jenkins96(k []byte, init uint32, init2 uint32) (uint32, uint32) {
	var o int
	l := uint32(len(k))
	a := 0xDEADBEEF + l + init
	b := 0xDEADBEEF + l + init
	c := 0xDEADBEEF + l + init2

	for l > 12 {
		a += uint32(k[o+0]) | uint32(k[o+1])<<8 | uint32(k[o+2])<<16 | uint32(k[o+3])<<24
		b += uint32(k[o+4]) | uint32(k[o+5])<<8 | uint32(k[o+6])<<16 | uint32(k[o+7])<<24
		c += uint32(k[o+8]) | uint32(k[o+9])<<8 | uint32(k[o+10])<<16 | uint32(k[o+11])<<24

		a -= c
		a ^= (c<<4 | c>>(32-4))
		c += b
		b -= a
		b ^= (a<<6 | a>>(32-6))
		a += c
		c -= b
		c ^= (b<<8 | b>>(32-8))
		b += a
		a -= c
		a ^= (c<<16 | c>>(32-16))
		c += b
		b -= a
		b ^= (a<<19 | a>>(32-19))
		a += c
		c -= b
		c ^= (b<<4 | b>>(32-4))
		b += a

		l -= 12
		o += 12
	}

	if l > 0 {
		switch l {
		case 12:
			c += uint32(k[o+11]) << 24
		case 11:
			c += uint32(k[o+10]) << 16
		case 10:
			c += uint32(k[o+9]) << 8
		case 9:
			c += uint32(k[o+8])
		case 8:
			b += uint32(k[o+7]) << 24
		case 7:
			b += uint32(k[o+6]) << 16
		case 6:
			b += uint32(k[o+5]) << 8
		case 5:
			b += uint32(k[o+4])
		case 4:
			a += uint32(k[o+3]) << 24
		case 3:
			a += uint32(k[o+2]) << 16
		case 2:
			a += uint32(k[o+1]) << 8
		case 1:
			a += uint32(k[o+0])
		}

		c ^= b
		c -= (b<<14 | b>>(32-14))
		a ^= c
		a -= (c<<11 | c>>(32-11))
		b ^= a
		b -= (a<<25 | a>>(32-25))
		c ^= b
		c -= (b<<16 | b>>(32-16))
		a ^= c
		a -= (c<<4 | c>>(32-4))
		b ^= a
		b -= (a<<14 | a>>(32-14))
		c ^= b
		c -= (b<<24 | b>>(32-24))
	}

	return b, c
}
