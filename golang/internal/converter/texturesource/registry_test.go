package texturesource

import "testing"

func TestReleaseGeneratedPNG(t *testing.T) {
	Clear()
	Register("skin.png", Source{Kind: KindPNG, PNG: []byte{1, 2, 3}})
	Register("cloak.png", Source{Kind: KindBLP, FileDataID: 42})

	if n := ReleaseGeneratedPNG([]string{"skin.png", "cloak.png"}); n != 1 {
		t.Fatalf("released %d png entries, want 1", n)
	}
	if _, ok := Get("skin.png"); ok {
		t.Fatal("skin.png should be released")
	}
	if _, ok := Get("cloak.png"); !ok {
		t.Fatal("blp entry should remain")
	}
}
