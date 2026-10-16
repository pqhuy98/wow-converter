package mdx

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestAnimatedTextureHasValidInitialMDXTexture(t *testing.T) {
	model := NewModel()
	if err := model.LoadMdl(`Version { FormatVersion 1000, }
Model "Baked pages" {}
Materials 1 {
    Material {
        Layer {
            FilterMode AddAlpha,
            TextureID 2 {
                DontInterp,
                0: 2,
                500: 3,
            }
        }
    }
}`); err != nil {
		t.Fatal(err)
	}
	layer := model.Materials[0].Layers[0]
	if layer.TextureID != 2 {
		t.Fatalf("initial texture = %d, want first animated page 2", layer.TextureID)
	}
	if len(layer.Animations) != 1 || layer.Animations[0].Values[1].([]uint32)[0] != 3 {
		t.Fatal("animated page switching was lost")
	}
	data := model.SaveMdx()
	material := bytes.Index(data, []byte("LAYS"))
	if material < 0 || int32(binary.LittleEndian.Uint32(data[material+20:material+24])) != 2 {
		t.Fatal("MDX static texture must initialize to page 2")
	}
}
