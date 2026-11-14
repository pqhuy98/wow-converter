package casc

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"io"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

func compressedBLTE(t *testing.T, framed bool, sizeDelta int, corrupt bool, chunks ...[]byte) ([]byte, string) {
	t.Helper()
	var blocks [][]byte
	for _, chunk := range chunks {
		var compressed bytes.Buffer
		compressed.WriteByte('Z')
		writer := zlib.NewWriter(&compressed)
		_, _ = writer.Write(chunk)
		_ = writer.Close()
		blocks = append(blocks, compressed.Bytes())
	}
	if corrupt {
		blocks[0][len(blocks[0])-1] ^= 1 // Adler checksum; BLTE MD5 remains valid.
	}
	headerSize := 0
	if framed {
		headerSize = 12 + len(chunks)*24
	}
	raw := make([]byte, max(8, headerSize))
	copy(raw, "BLTE")
	binary.BigEndian.PutUint32(raw[4:], uint32(headerSize))
	if framed {
		raw[8], raw[11] = 15, byte(len(chunks))
		for i, block := range blocks {
			record := raw[12+i*24:]
			binary.BigEndian.PutUint32(record, uint32(len(block)))
			binary.BigEndian.PutUint32(record[4:], uint32(len(chunks[i])+sizeDelta))
			hash := md5.Sum(block)
			copy(record[8:], hash[:])
		}
	}
	for _, block := range blocks {
		raw = append(raw, block...)
	}
	if !framed {
		headerSize = len(raw)
	}
	hash := md5.Sum(raw[:headerSize])
	return raw, hex.EncodeToString(hash[:])
}

func TestBLTEDirectInflation(t *testing.T) {
	chunks := [][]byte{bytes.Repeat([]byte("first"), 10000), bytes.Repeat([]byte("second"), 20000)}
	for _, framed := range []bool{false, true} {
		parts := chunks
		if !framed {
			parts = [][]byte{bytes.Join(chunks, nil)}
		}
		raw, hash := compressedBLTE(t, framed, 0, false, parts...)
		reader, err := NewBLTEReader(buffer.From(raw), hash, false)
		if err != nil {
			t.Fatal(err)
		}
		if framed {
			reader.CheckBounds(8)
			if reader.blockIndex != 1 {
				t.Fatal("lost lazy block decoding")
			}
		}
		if err := reader.ProcessAllBlocks(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(reader.Raw(), bytes.Join(parts, nil)) {
			t.Fatal("decoded bytes changed")
		}
	}
}

func TestBLTERejectsBadDecodedSizeAndChecksum(t *testing.T) {
	for _, delta := range []int{-1, 0, 1} {
		raw, hash := compressedBLTE(t, true, delta, delta == 0, []byte("block payload"))
		reader, err := NewBLTEReader(buffer.From(raw), hash, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := reader.ProcessAllBlocks(); err == nil {
			t.Fatalf("accepted corrupt block, delta=%d", delta)
		}
	}
}

func TestRealBLTEInflationParity(t *testing.T) {
	raw, hash := realEncodingFixture(t)
	reader, err := NewBLTEReader(buffer.From(raw), hash, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.ProcessAllBlocks(); err != nil {
		t.Fatal(err)
	}
	want := md5.New()
	offset := int(binary.BigEndian.Uint32(raw[4:8]))
	for _, block := range reader.blocks {
		if raw[offset] == 'N' {
			_, _ = want.Write(raw[offset+1 : offset+block.CompSize])
			offset += block.CompSize
			continue
		}
		if raw[offset] != 'Z' {
			t.Fatalf("fixture has unexpected block flag %q", raw[offset])
		}
		inflater, err := zlib.NewReader(bytes.NewReader(raw[offset+1 : offset+block.CompSize]))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(want, inflater); err != nil {
			t.Fatal(err)
		}
		_ = inflater.Close()
		offset += block.CompSize
	}
	got := md5.Sum(reader.Raw())
	if !bytes.Equal(got[:], want.Sum(nil)) {
		t.Fatal("real Blizzard BLTE decoded content changed")
	}
	t.Logf("verified %d decoded bytes across %d blocks", reader.ByteLength(), len(reader.blocks))
}
