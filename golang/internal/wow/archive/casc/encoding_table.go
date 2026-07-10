package casc

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
}

func (t *EncodingTable) init(contentKeyLen, encodingKeyLen int) {
	t.contentKeyLen = contentKeyLen
	t.encodingKeyLen = encodingKeyLen
	t.entries = make(map[Key16]encodingEntry)
}

func (t *EncodingTable) set(contentKey CascKey, encodingKey CascKey, size int) {
	t.entries[key16FromCascKey(contentKey, t.contentKeyLen)] = encodingEntry{
		encKey: key16FromCascKey(encodingKey, t.encodingKeyLen),
		size:   size,
	}
}

func (t *EncodingTable) lookup(contentKey CascKey) (encodingKey CascKey, size int, ok bool) {
	entry, ok := t.entries[key16FromCascKey(contentKey, t.contentKeyLen)]
	if !ok {
		return "", 0, false
	}
	return key16ToCascKey(entry.encKey, t.encodingKeyLen), entry.size, true
}

func (t *EncodingTable) lookupEncodingKey(contentKey CascKey) (encodingKey CascKey, ok bool) {
	entry, ok := t.entries[key16FromCascKey(contentKey, t.contentKeyLen)]
	if !ok {
		return "", false
	}
	return key16ToCascKey(entry.encKey, t.encodingKeyLen), true
}

func (t *EncodingTable) len() int {
	return len(t.entries)
}
