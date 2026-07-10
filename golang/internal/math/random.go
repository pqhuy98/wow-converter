package math

// Rand returns floats in [0, 1).
type Rand func() float64

// Mulberry32 is a fast 32-bit seeded PRNG.
func Mulberry32(seed uint32) Rand {
	a := seed
	return func() float64 {
		a = a + 0x6D2B79F5
		t := a
		t = imul32(t^(t>>15), t|1)
		t ^= t + imul32(t^(t>>7), t|61)
		return float64((t^(t>>14))&0xFFFFFFFF) / 4294967296
	}
}

func imul32(a, b uint32) uint32 {
	return uint32(int32(a) * int32(b))
}

// HashStringToSeed is an FNV-1a 32-bit string hash for stable seeds.
func HashStringToSeed(str string) uint32 {
	h := uint32(0x811C9DC5)
	for i := 0; i < len(str); i++ {
		h ^= uint32(str[i])
		h = imul32(h, 0x01000193)
	}
	return h
}

// SeededRandom returns a PRNG derived from a string key.
func SeededRandom(key string) Rand {
	return Mulberry32(HashStringToSeed(key))
}
