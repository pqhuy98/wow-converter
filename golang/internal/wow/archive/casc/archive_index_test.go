package casc

import (
	"encoding/binary"
	"encoding/hex"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestArchiveIndexPaddingAndBounds(t *testing.T) {
	data := make([]byte, 4096+24+28)
	for i := 0; i < 171; i++ {
		pos := i * 24
		if i == 170 {
			pos = 4096
		}
		binary.BigEndian.PutUint32(data[pos+12:], uint32(i+1))
		binary.BigEndian.PutUint32(data[pos+16:], uint32(i+10))
		binary.BigEndian.PutUint32(data[pos+20:], uint32(i+20))
	}
	binary.LittleEndian.PutUint32(data[len(data)-12:], 171)
	entries, err := decodeArchiveIndex(data)
	if err != nil || len(entries) != 171 {
		t.Fatalf("decode: %d %v", len(entries), err)
	}
	if entries[170].size != 180 || entries[170].offset != 190 {
		t.Fatal(entries[170])
	}
	for _, bad := range [][]byte{nil, make([]byte, 11), append(make([]byte, 12), 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0)} {
		if _, err := decodeArchiveIndex(bad); err == nil {
			t.Fatal("accepted invalid index")
		}
	}
}

type archiveFixture struct {
	key  string
	data []byte
}

func realArchiveFixtures(tb testing.TB) []archiveFixture {
	tb.Helper()
	dir := os.Getenv("CASC_INDEX_BENCH_DIR")
	if dir == "" {
		tb.Skip("set CASC_INDEX_BENCH_DIR for real cached CDN indexes")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.index"))
	if err != nil {
		tb.Fatal(err)
	}
	if len(files) == 0 {
		tb.Fatal("no archive indexes")
	}
	fixtures := make([]archiveFixture, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			tb.Fatal(err)
		}
		fixtures = append(fixtures, archiveFixture{filepath.Base(file), data})
	}
	return fixtures
}

// Reference reproduces the previous map/string expansion, independently of the
// new binary decoder, for both real-data parity and allocation benchmarks.
func legacyArchiveDecode(fixtures []archiveFixture) map[string]archiveEntry {
	all := map[string]archiveEntry{}
	for _, f := range fixtures {
		count := int(binary.LittleEndian.Uint32(f.data[len(f.data)-12:]))
		pos := 0
		entries := make(map[string]archiveEntry, count)
		for i := 0; i < count; i++ {
			key := hex.EncodeToString(f.data[pos : pos+16])
			pos += 16
			if key == emptyHash {
				key = hex.EncodeToString(f.data[pos : pos+16])
				pos += 16
			}
			entries[key] = archiveEntry{f.key, int(binary.BigEndian.Uint32(f.data[pos:])), int(binary.BigEndian.Uint32(f.data[pos+4:]))}
			pos += 8
		}
		for key, e := range entries {
			all[key] = e
		}
	}
	return all
}

func compactArchiveDecode(tb testing.TB, fixtures []archiveFixture) (map[Key16]archiveLocation, []string) {
	tb.Helper()
	all := map[Key16]archiveLocation{}
	keys := make([]string, len(fixtures))
	for i, f := range fixtures {
		keys[i] = f.key
		entries, err := decodeArchiveIndex(f.data)
		if err != nil {
			tb.Fatalf("%s: %v", f.key, err)
		}
		for _, e := range entries {
			all[e.key] = archiveLocation{uint32(i), e.size, e.offset}
		}
	}
	return all, keys
}

func TestRealArchiveIndexParity(t *testing.T) {
	fixtures := realArchiveFixtures(t)
	legacy := legacyArchiveDecode(fixtures)
	compact, keys := compactArchiveDecode(t, fixtures)
	if len(legacy) != len(compact) {
		t.Fatalf("count: %d vs %d", len(legacy), len(compact))
	}
	for key, want := range legacy {
		var binaryKey Key16
		hex.Decode(binaryKey[:], []byte(key))
		got, ok := compact[binaryKey]
		if !ok || int(got.size) != want.Size || int(got.offset) != want.Offset || keys[got.archiveID] != want.Key {
			t.Fatalf("mismatch %s", key)
		}
	}
	t.Logf("matched %d entries across %d real archive indexes", len(legacy), len(fixtures))
}

var archiveBenchSink any

func TestRealIndexRetainedHeap(t *testing.T) {
	fixtures := realArchiveFixtures(t)
	measure := func(name string, load func() any) {
		archiveBenchSink = nil
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		archiveBenchSink = load()
		runtime.GC()
		runtime.ReadMemStats(&after)
		t.Logf("%s retained heap %.2f MiB", name, float64(after.HeapAlloc-before.HeapAlloc)/(1024*1024))
	}
	measure("archive legacy", func() any { return legacyArchiveDecode(fixtures) })
	measure("archive compact", func() any { table, keys := compactArchiveDecode(t, fixtures); return []any{table, keys} })
	runtime.KeepAlive(fixtures)
	data, hash := realEncodingFixture(t)
	measure("encoding legacy", func() any {
		c := NewBaseCASC(true)
		if err := legacyParseEncodingFile(c, buffer.NewBuffer(data), hash); err != nil {
			t.Fatal(err)
		}
		return c
	})
	measure("encoding native", func() any {
		c := NewBaseCASC(true)
		if err := c.ParseEncodingFile(buffer.NewBuffer(data), hash); err != nil {
			t.Fatal(err)
		}
		return c
	})
	runtime.KeepAlive(data)
	archiveBenchSink = nil
}

var archiveLookupSink archiveEntry
var archiveLookupOK bool

func BenchmarkRealIndexLookups(b *testing.B) {
	f := realArchiveFixtures(b)
	old := legacyArchiveDecode(f)
	table, keys := compactArchiveDecode(b, f)
	source := NewCASCRemote("eu")
	source.archives = table
	source.archiveKeys = keys
	var text []string
	var binaryKeys []Key16
	for key := range old {
		var k Key16
		hex.Decode(k[:], []byte(key))
		text = append(text, key)
		binaryKeys = append(binaryKeys, k)
		if len(text) == 1024 {
			break
		}
	}
	b.Run("archive_legacy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			archiveLookupSink, archiveLookupOK = old[text[i%len(text)]]
		}
	})
	b.Run("archive_compact", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			archiveLookupSink, archiveLookupOK = source.lookupArchiveKey(binaryKeys[i%len(text)])
		}
	})
	data, hash := realEncodingFixture(b)
	legacy := NewBaseCASC(true)
	native := NewBaseCASC(true)
	if err := legacyParseEncodingFile(legacy, buffer.NewBuffer(data), hash); err != nil {
		b.Fatal(err)
	}
	if err := native.ParseEncodingFile(buffer.NewBuffer(data), hash); err != nil {
		b.Fatal(err)
	}
	var contentKeys []CascKey
	for key := range legacy.EncodingTable.entries {
		contentKeys = append(contentKeys, CascKey(string(key[:])))
		if len(contentKeys) == 1024 {
			break
		}
	}
	var k CascKey
	var size int
	var ok bool
	b.Run("encoding_legacy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			k, size, ok = legacy.EncodingTable.lookup(contentKeys[i%len(contentKeys)])
		}
	})
	b.Run("encoding_native", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			k, size, ok = native.EncodingTable.lookup(contentKeys[i%len(contentKeys)])
		}
	})
	if !ok || len(k) == 0 || size < 0 {
		b.Fatal("invalid result")
	}
}

func BenchmarkRealArchiveLoad(b *testing.B) {
	f := realArchiveFixtures(b)
	b.Run("legacy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			archiveBenchSink = legacyArchiveDecode(f)
		}
	})
	b.Run("compact", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			table, keys := compactArchiveDecode(b, f)
			archiveBenchSink = []any{table, keys}
		}
	})
}
