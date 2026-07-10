package mdx

import "testing"

func TestTextureEmptyImageReadMdl(t *testing.T) {
	text := `Textures 1 {
	Bitmap {
		Image "",
	}
}`
	m := NewModel()
	if err := m.LoadMdl(text); err != nil {
		t.Fatal(err)
	}
	if len(m.Textures) != 1 {
		t.Fatalf("textures=%d", len(m.Textures))
	}
}

func TestTextureReplaceableReadMdl(t *testing.T) {
	text := `Textures 1 {
	Bitmap {
		ReplaceableId 11,
	}
}`
	m := NewModel()
	if err := m.LoadMdl(text); err != nil {
		t.Fatal(err)
	}
}
