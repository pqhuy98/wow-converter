package casc

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// Keep binary hashes and one archive ID per location, rather than allocating a
// hex string and repeating a string header plus two machine-sized ints per file.
type archiveLocation struct{ archiveID, size, offset uint32 }
type archiveIndexEntry struct {
	key          Key16
	size, offset uint32
}

func decodeArchiveIndex(data []byte) ([]archiveIndexEntry, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("archive index truncated: %d bytes", len(data))
	}
	count := uint64(binary.LittleEndian.Uint32(data[len(data)-12:]))
	if count > uint64(len(data))/24 {
		return nil, fmt.Errorf("archive index count exceeds size")
	}
	entries := make([]archiveIndexEntry, 0, int(count))
	pos := 0
	for i := uint64(0); i < count; i++ {
		if pos+16 > len(data) {
			return nil, fmt.Errorf("archive index truncated key")
		}
		var key Key16
		copy(key[:], data[pos:pos+16])
		pos += 16
		if key == (Key16{}) {
			// Native 4 KiB pages have 16 padding bytes after 170 records.
			if pos+16 > len(data) {
				return nil, fmt.Errorf("archive index truncated after padding")
			}
			copy(key[:], data[pos:pos+16])
			pos += 16
		}
		if pos+8 > len(data) {
			return nil, fmt.Errorf("archive index truncated location")
		}
		entries = append(entries, archiveIndexEntry{key: key,
			size: binary.BigEndian.Uint32(data[pos:]), offset: binary.BigEndian.Uint32(data[pos+4:])})
		pos += 8
	}
	return entries, nil
}

// Caller holds archivesMu while adding an archive not supplied by LoadArchives.
func (c *CASCRemote) archiveID(key string) uint32 {
	for i, existing := range c.archiveKeys {
		if existing == key {
			return uint32(i)
		}
	}
	c.archiveKeys = append(c.archiveKeys, key)
	return uint32(len(c.archiveKeys) - 1)
}

func (c *CASCRemote) lookupArchive(key string) (archiveEntry, bool) {
	var hash Key16
	if len(key) != 32 {
		return archiveEntry{}, false
	}
	if _, err := hex.Decode(hash[:], []byte(key)); err != nil {
		return archiveEntry{}, false
	}
	return c.lookupArchiveKey(hash)
}

func (c *CASCRemote) lookupArchiveKey(hash Key16) (archiveEntry, bool) {
	c.archivesMu.RLock()
	loc, ok := c.archives[hash]
	var result archiveEntry
	if ok {
		result = archiveEntry{Key: c.archiveKeys[loc.archiveID], Size: int(loc.size), Offset: int(loc.offset)}
	}
	c.archivesMu.RUnlock()
	return result, ok
}
