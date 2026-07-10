// Package buffer provides a binary reader/writer and checksum helpers.
package buffer

// CRC32 lookup table for checksum calculation.
var crc32Table [256]uint32

func init() {
	for i := 0; i < 256; i++ {
		current := uint32(i)
		for j := 0; j < 8; j++ {
			if current&1 != 0 {
				current = 0xEDB88320 ^ (current >> 1)
			} else {
				current >>= 1
			}
		}
		crc32Table[i] = current
	}
}

// CRC32 calculates the CRC32 value of the given buffer.
func CRC32(buf []byte) uint32 {
	var res uint32 = 0xFFFFFFFF
	for i := 0; i < len(buf); i++ {
		res = crc32Table[(res^uint32(buf[i]))&0xFF] ^ (res >> 8)
	}
	return res ^ 0xFFFFFFFF
}
