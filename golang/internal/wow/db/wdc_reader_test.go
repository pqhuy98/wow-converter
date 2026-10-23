package db

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

type inlineFieldTestSource struct{ RuntimeFileSource }

func (inlineFieldTestSource) GetBuildName() string { return "1.0.0.1" }
func (inlineFieldTestSource) GetDBD(context.Context, string) ([]byte, error) {
	return []byte("COLUMNS\nint ID\nint DoodadID\nint Sound\n\nLAYOUT 12345678\nBUILD 1.0.0.1\n$id$ID<u8>\nDoodadID<u16>[4]\nSound<u8>\n"), nil
}

func TestNormalInlineArrayAfterPackedFieldUsesOffset(t *testing.T) {
	var b bytes.Buffer
	u32 := func(v uint32) {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	u16 := func(v uint16) {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	// One normal WDC3 record. A packed ID is followed in schema order by
	// an inline uint16 array, then an earlier inline byte in record order.
	for _, v := range []uint32{0x33434457, 1, 3, 16, 0, 0, 0x12345678, 7, 7, 0} {
		u32(v)
	}
	u16(0)
	u16(0)
	for _, v := range []uint32{3, 0, 0, 72, 0, 0, 1} {
		u32(v)
	}
	const recordOffset = 72 + 40 + 12 + 72
	for _, v := range []uint32{0, 0, recordOffset, 1, 0, recordOffset + 16, 0, 0, 0, 0} {
		u32(v)
	}
	for i := 0; i < 3; i++ {
		u16(0)
		u16(0)
	}
	for _, f := range []struct {
		offset, size uint16
		compression  uint32
	}{{0, 8, uint32(CompressionBitpacked)}, {16, 64, 0}, {8, 8, 0}} {
		u16(f.offset)
		u16(f.size)
		u32(0)
		u32(f.compression)
		u32(0)
		u32(0)
		u32(0)
	}
	b.Write([]byte{7, 9})
	for _, v := range []uint16{100, 200, 300, 400} {
		u16(v)
	}
	b.Write(make([]byte, 6))
	r := NewWDCReader("InlineFixture.db2", inlineFieldTestSource{})
	if err := r.Parse(context.Background(), b.Bytes()); err != nil {
		t.Fatal(err)
	}
	row := r.GetRow(7)
	if !reflect.DeepEqual(row["DoodadID"], []int64{100, 200, 300, 400}) || row["Sound"] != int64(9) {
		t.Fatalf("inline fields read from stale cursor: %+v", row)
	}
}

func TestConvertDBDToSchemaTypePreservesFloatArrayLength(t *testing.T) {
	entry := &DBDField{Type: DBDColumnFloat, ArrayLength: 3}
	schema := convertDBDToSchemaType(entry)
	if schema.Type != FieldTypeFloat || schema.Count != 3 {
		t.Fatalf("float array schema = %+v", schema)
	}
}

func TestReinterpretCompressedFloatArray(t *testing.T) {
	raw := []uint32{math.Float32bits(1.25), math.Float32bits(-2.5)}
	got := reinterpretCompressedFloatArray(buffer.Alloc(8, true), raw)
	if !reflect.DeepEqual(got, []float32{1.25, -2.5}) {
		t.Fatalf("reinterpretCompressedFloatArray() = %v", got)
	}
}

func TestCompressedNarrowIntegerArrayValues(t *testing.T) {
	for _, tc := range []struct {
		kind FieldType
		raw  uint32
		want uint64
	}{{FieldTypeUInt8, 82, 82}, {FieldTypeUInt16, 2188, 2188}, {FieldTypeInt8, 0xff, ^uint64(0)}, {FieldTypeInt16, 0xffff, ^uint64(0)}} {
		b := buffer.Alloc(8, true)
		b.WriteBigUInt64LE(uint64(tc.raw))
		b.Seek(0)
		if got := toUint64(reinterpretField(b, tc.kind)); got != tc.want {
			t.Fatalf("compressed type %d value %d became %d", tc.kind, tc.raw, got)
		}
	}
}
