package casc

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"os"
	"testing"
)

func TestEncodingNativePages(t *testing.T) {
	table := newEncodingTable(4, 4)
	table.entries = nil
	table.pages = []encodingPage{}
	for _, id := range []uint32{1, 100} {
		data := make([]byte, 32)
		data[0] = 2
		data[5] = 99
		binary.BigEndian.PutUint32(data[6:], id)
		copy(data[10:], []byte{1, 2, 3, 4})
		copy(data[14:], []byte{5, 6, 7, 8})
		if err := table.addPage(data); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []uint32{1, 100} {
		var k [4]byte
		binary.BigEndian.PutUint32(k[:], id)
		key, size, ok := table.lookup(CascKey(string(k[:])))
		if !ok || size != 99 || key != CascKey("\x01\x02\x03\x04") {
			t.Fatal(key, size, ok)
		}
	}
	if _, _, ok := table.lookup(CascKey("\x00\x00\x00\x02")); ok {
		t.Fatal("false hit")
	}
	if table.len() != 2 {
		t.Fatal(table.len())
	}
	if err := table.addPage([]byte{1}); err == nil {
		t.Fatal("accepted truncated entry")
	}
	table.reset()
	if table.len() != 0 {
		t.Fatal("reset retained pages")
	}
}

func realEncodingFixture(tb testing.TB) ([]byte, string) {
	path := os.Getenv("CASC_ENCODING_BENCH_FILE")
	if path == "" {
		tb.Skip("set CASC_ENCODING_BENCH_FILE")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatal(err)
	}
	end := int(binary.BigEndian.Uint32(data[4:8]))
	if end == 0 {
		end = len(data)
	}
	hash := md5.Sum(data[:end])
	return data, hex.EncodeToString(hash[:])
}

func TestRealEncodingNativeParity(t *testing.T) {
	data, hash := realEncodingFixture(t)
	legacy := NewBaseCASC(true)
	native := NewBaseCASC(true)
	if err := legacyParseEncodingFile(legacy, buffer.NewBuffer(data), hash); err != nil {
		t.Fatal(err)
	}
	if err := native.ParseEncodingFile(buffer.NewBuffer(data), hash); err != nil {
		t.Fatal(err)
	}
	if legacy.EncodingTable.len() != native.EncodingTable.len() {
		t.Fatalf("counts %d vs %d", legacy.EncodingTable.len(), native.EncodingTable.len())
	}
	for key, want := range legacy.EncodingTable.entries {
		got, size, ok := native.EncodingTable.lookup(CascKey(string(key[:])))
		if !ok || got != key16ToCascKey(want.encKey, 16) || size != want.size {
			t.Fatalf("mismatch %x", key)
		}
	}
	t.Logf("matched all %d content keys in %d native pages", native.EncodingTable.len(), len(native.EncodingTable.pages))
}

func BenchmarkRealEncodingLoad(b *testing.B) {
	data, hash := realEncodingFixture(b)
	for _, tc := range []struct {
		name string
		load func(*BaseCASC, *buffer.Buffer, string) error
	}{{"legacy", legacyParseEncodingFile}, {"native", func(c *BaseCASC, d *buffer.Buffer, h string) error { return c.ParseEncodingFile(d, h) }}} {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				c := NewBaseCASC(true)
				if err := tc.load(c, buffer.NewBuffer(data), hash); err != nil {
					b.Fatal(err)
				}
				archiveBenchSink = c
			}
		})
	}
}
