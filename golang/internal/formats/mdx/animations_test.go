package mdx

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestAnimationGlobalSequenceIDDefaultsToMinusOne(t *testing.T) {
	a, _, err := newAnimationForTag("KGRT")
	if err != nil {
		t.Fatal(err)
	}
	if a.GlobalSequenceID != -1 {
		t.Fatalf("new animation GlobalSequenceID=%d want -1", a.GlobalSequenceID)
	}

	text := `Version {
FormatVersion 1000,
}
Model "Test" {
}
Bone "Root" {
ObjectId 0,

Rotation 2 {

Linear,

0: { 0, 0, 0, 1 },
100: { 0, 0, 0, 1 },
}
}`
	m := NewModel()
	if err := m.LoadMdl(text); err != nil {
		t.Fatal(err)
	}
	if len(m.Bones) != 1 || len(m.Bones[0].Animations) != 1 {
		t.Fatalf("bones=%d anims=%d", len(m.Bones), len(m.Bones[0].Animations))
	}
	if got := m.Bones[0].Animations[0].GlobalSequenceID; got != -1 {
		t.Fatalf("LoadMdl GlobalSequenceID=%d want -1", got)
	}

	data := m.SaveMdx()
	off := bytes.Index(data, []byte("KGRT"))
	if off < 0 {
		t.Fatal("missing KGRT chunk")
	}
	gs := int32(binary.LittleEndian.Uint32(data[off+12 : off+16]))
	if gs != -1 {
		t.Fatalf("mdx globalSequenceId=%d want -1", gs)
	}
}
