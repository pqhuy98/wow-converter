package mdx

import (
	"bytes"
	"strings"
	"testing"
)

func TestModelBinaryAndTextRoundTrip(t *testing.T) {
	m := NewModel()
	if err := m.LoadMdl(`Version { FormatVersion 1000, }
 Model "round trip" { BlendTime 150, MinimumExtent { -1, -2, -3 }, MaximumExtent { 1, 2, 3 }, BoundsRadius 4, }
 Sequences 1 { Anim "Stand 1" { Interval { 0, 1000 }, } }
 GlobalSequences 1 { Duration 1000, }
 Textures 1 { Bitmap { Image "wow/test.blp", } }
 Materials 1 { Material { Layer { FilterMode None, static TextureID 0, } } }
 Bone "root" { ObjectId 0, GeosetId Multiple, GeosetAnimId None, Translation 2 { Linear, 0: { 0, 0, 0 }, 1000: { 1, 2, 3 }, } }
 PivotPoints 1 { { 0, 0, 0 }, }
 BindPose { Matrices 1 { { 1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0 }, } }`); err != nil {
		t.Fatal(err)
	}
	raw := m.SaveMdx()
	decoded := NewModel()
	if err := decoded.LoadMdx(raw); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, decoded.SaveMdx()) {
		t.Fatal("binary round trip changed fields")
	}
	text := NewModel()
	if err := text.LoadMdl(decoded.SaveMdl()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, text.SaveMdx()) {
		t.Fatal("text round trip changed fields")
	}
	decoded.UnknownChunks = []*UnknownChunk{{Tag: "TEST", Chunk: []byte{1, 2, 3}}}
	withUnknown := decoded.SaveMdx()
	if err := m.LoadMdx(withUnknown); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(withUnknown, m.SaveMdx()) {
		t.Fatal("unknown chunk lost")
	}
	for _, bad := range [][]byte{nil, []byte("MDLXBAD"), append([]byte("MDLXVERS"), 255, 255, 255, 255)} {
		if err := m.LoadMdx(bad); err == nil {
			t.Fatal("accepted truncated MDX")
		}
	}
}

func TestLongModelNameDoesNotBecomeExternalAnimationFile(t *testing.T) {
	m := NewModel()
	m.Name = strings.Repeat("model/", 30)
	decoded := NewModel()
	if err := decoded.LoadMdx(m.SaveMdx()); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Name) != 80 || decoded.AnimationFile != "" {
		t.Fatal("name spilled into animation file field")
	}
}
