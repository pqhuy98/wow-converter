package components

import (
	"strings"
	"testing"
)

func TestTexturesToStringDoesNotInferReplaceableIDFromWowType(t *testing.T) {
	out := TexturesToString([]*Texture{{
		Image:   "",
		WowData: TextureWowData{Type: 11},
	}})
	if strings.Contains(out, "ReplaceableId") {
		t.Fatalf("expected no inferred ReplaceableId for unresolved WoW texture type, got:\n%s", out)
	}
}

func TestTexturesToStringWritesExplicitReplaceableID(t *testing.T) {
	id := 11
	out := TexturesToString([]*Texture{{
		Image:         "",
		ReplaceableID: &id,
		WowData:       TextureWowData{Type: 0},
	}})
	if !strings.Contains(out, "ReplaceableId 11") {
		t.Fatalf("expected explicit ReplaceableId to be written, got:\n%s", out)
	}
}
