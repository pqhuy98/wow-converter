package db

import (
	"math"
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

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
