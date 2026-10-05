package m2

import (
	"context"
	"encoding/binary"
	"testing"
)

func TestSkinBatchPriorityIsSigned(t *testing.T) {
	priorities := []int8{-128, -27, -17, 0, 20, 127}
	raw := make([]byte, 48+24*len(priorities))
	binary.LittleEndian.PutUint32(raw, magicSkin)
	binary.LittleEndian.PutUint32(raw[36:], uint32(len(priorities)))
	binary.LittleEndian.PutUint32(raw[40:], 48)
	for i, priority := range priorities {
		raw[48+i*24+1] = byte(priority)
	}
	skin := &Skin{getFile: func(context.Context, uint32) ([]byte, error) { return raw, nil }}
	if err := skin.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i, priority := range priorities {
		if skin.TextureUnits[i].Priority != priority {
			t.Fatalf("batch %d: priority %d, want %d", i, skin.TextureUnits[i].Priority, priority)
		}
	}
}
