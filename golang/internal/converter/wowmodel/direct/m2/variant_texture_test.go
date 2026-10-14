package directm2

import "testing"

func TestVariantTextureForType(t *testing.T) {
	textures := []int{101, 102, 103, 104}
	for kind, want := range map[int]int{0: 0, 1: 0, 2: 101, 3: 102, 4: 103, 5: 104, 11: 101, 12: 102, 13: 103, 14: 0} {
		if got := VariantTextureForType(textures, kind); got != want {
			t.Errorf("type %d: got %d, want %d", kind, got, want)
		}
	}
	if VariantTextureForType([]int{101}, 12) != 0 {
		t.Fatal("missing skin slots must not read another texture")
	}
}
