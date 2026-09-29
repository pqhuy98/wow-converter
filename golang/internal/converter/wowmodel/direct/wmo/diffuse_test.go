package directwmo

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

func TestWmoDiffuseFileIDShader23(t *testing.T) {
	// 12tr_amani_hub03 material 5: texture1 is empty, texture2 is the roof, texture3 is a mask.
	roof := wmo.Material{
		Shader: 23, Texture2: 6029198, Texture3: 6100252, Flags3: 5688800,
		RuntimeData: []uint32{0, 0, 0, 0},
	}
	if got := wmoDiffuseFileID(roof); got != 6029198 {
		t.Fatalf("empty texture1 bound %d, want texture2 6029198", got)
	}

	withFirst := wmo.Material{
		Shader: 23, Texture1: 1, Texture2: 2, Texture3: 3,
		RuntimeData: []uint32{0, 0, 0, 0},
	}
	if got := wmoDiffuseFileID(withFirst); got != 2 {
		t.Fatalf("shader 23 with texture1 bound %d, want 2", got)
	}

	plain := wmo.Material{Shader: 0, Texture1: 42, Texture2: 99}
	if got := wmoDiffuseFileID(plain); got != 42 {
		t.Fatalf("shader 0 bound %d, want texture1 42", got)
	}
}
