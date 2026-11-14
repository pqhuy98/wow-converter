package casc

import (
	"bytes"
	"fmt"
	"sort"
)

// Key16 is a fixed-size CASC key used as a compact map key.
type Key16 [16]byte

func key16FromCascKey(key CascKey, keyLen int) Key16 {
	var k Key16
	if keyLen > len(k) {
		keyLen = len(k)
	}
	copy(k[:], []byte(key)[:keyLen])
	return k
}

func key16ToCascKey(key Key16, keyLen int) CascKey {
	if keyLen <= 0 || keyLen > len(key) {
		keyLen = len(key)
	}
	return CascKey(key[:keyLen])
}

type encodingEntry struct {
	encKey Key16
	size   int
}

// EncodingTable maps content keys to encoding keys and decompressed sizes.
type EncodingTable struct {
	contentKeyLen  int
	encodingKeyLen int
	entries        map[Key16]encodingEntry
	pages          []encodingPage
	entryCount     int
}

// Native encoding pages stay packed. Only record offsets are indexed; no Go
// hash-map entry, string, or object is allocated for each content key.
type encodingPage struct {
	first   Key16
	data    []byte
	offsets []uint32
}

func newEncodingTable(contentKeyLen, encodingKeyLen int) *EncodingTable {
	return &EncodingTable{
		contentKeyLen:  contentKeyLen,
		encodingKeyLen: encodingKeyLen,
		entries:        make(map[Key16]encodingEntry),
	}
}

func (t *EncodingTable) reset() {
	t.contentKeyLen = 0
	t.encodingKeyLen = 0
	t.entries = make(map[Key16]encodingEntry)
	t.pages = nil
	t.entryCount = 0
}

func (t *EncodingTable) init(contentKeyLen, encodingKeyLen int) {
	t.contentKeyLen = contentKeyLen
	t.encodingKeyLen = encodingKeyLen
	t.entries = make(map[Key16]encodingEntry)
	t.pages = nil
	t.entryCount = 0
}

func (t *EncodingTable) set(contentKey CascKey, encodingKey CascKey, size int) {
	t.entries[key16FromCascKey(contentKey, t.contentKeyLen)] = encodingEntry{
		encKey: key16FromCascKey(encodingKey, t.encodingKeyLen),
		size:   size,
	}
}

func (t *EncodingTable) lookup(contentKey CascKey) (encodingKey CascKey, size int, ok bool) {
	if t.pages != nil {
		return t.lookupPage(contentKey)
	}
	entry, ok := t.entries[key16FromCascKey(contentKey, t.contentKeyLen)]
	if !ok {
		return "", 0, false
	}
	return key16ToCascKey(entry.encKey, t.encodingKeyLen), entry.size, true
}

func (t *EncodingTable) lookupEncodingKey(contentKey CascKey) (encodingKey CascKey, ok bool) {
	encodingKey, _, ok = t.lookup(contentKey)
	return encodingKey, ok
}

func (t *EncodingTable) len() int {
	if t.pages != nil {
		return t.entryCount
	}
	return len(t.entries)
}

func (t *EncodingTable) addPage(data []byte) error {
	p := encodingPage{data: data, offsets: make([]uint32, 0, len(data)/(6+t.contentKeyLen+t.encodingKeyLen))}
	var last Key16
	for pos := 0; pos < len(data); {
		n := int(data[pos])
		if n == 0 {
			break
		}
		end := pos + 6 + t.contentKeyLen + n*t.encodingKeyLen
		if end > len(data) {
			return fmt.Errorf("encoding entry exceeds native page")
		}
		var key Key16
		copy(key[:], data[pos+6:pos+6+t.contentKeyLen])
		if len(p.offsets) > 0 && bytes.Compare(last[:], key[:]) >= 0 {
			return fmt.Errorf("encoding page keys out of order")
		}
		if len(p.offsets) == 0 {
			p.first = key
		}
		last = key
		p.offsets = append(p.offsets, uint32(pos))
		pos = end
	}
	if len(p.offsets) > 0 {
		if len(t.pages) > 0 && bytes.Compare(t.pages[len(t.pages)-1].first[:], p.first[:]) >= 0 {
			return fmt.Errorf("encoding pages out of order")
		}
		t.entryCount += len(p.offsets)
		t.pages = append(t.pages, p)
	}
	return nil
}

func (t *EncodingTable) lookupPage(contentKey CascKey) (CascKey, int, bool) {
	if len(contentKey) != t.contentKeyLen {
		return "", 0, false
	}
	key := []byte(contentKey)
	i := sort.Search(len(t.pages), func(i int) bool { return bytes.Compare(t.pages[i].first[:t.contentKeyLen], key) > 0 }) - 1
	if i < 0 {
		return "", 0, false
	}
	p := &t.pages[i]
	j := sort.Search(len(p.offsets), func(j int) bool {
		pos := int(p.offsets[j]) + 6
		return bytes.Compare(p.data[pos:pos+t.contentKeyLen], key) >= 0
	})
	if j == len(p.offsets) {
		return "", 0, false
	}
	pos := int(p.offsets[j])
	if !bytes.Equal(p.data[pos+6:pos+6+t.contentKeyLen], key) {
		return "", 0, false
	}
	size := uint64(0)
	for _, v := range p.data[pos+1 : pos+6] {
		size = size<<8 | uint64(v)
	}
	start := pos + 6 + t.contentKeyLen
	return CascKey(string(p.data[start : start+t.encodingKeyLen])), int(size), true
}
