package utils

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestWmoBlendModeToWc3FilterMode(t *testing.T) {
	tests := []struct {
		blend uint16
		want  components.BlendMode
	}{
		{0, components.BlendNone},
		{1, components.BlendTransparent},
		{2, components.BlendBlend},
		{3, components.BlendAdditive},
		{4, components.BlendModulate},
		{5, components.BlendModulate2x},
		{8, components.BlendBlend},
		{10, components.BlendAdditive},
		{99, components.BlendBlend},
	}
	for _, tt := range tests {
		if got := WmoBlendModeToWc3FilterMode(tt.blend); got != tt.want {
			t.Fatalf("blend %d: got %q want %q", tt.blend, got, tt.want)
		}
	}
}

func TestGuessFilterModeFloorIsNone(t *testing.T) {
	if got := GuessFilterMode("dungeons/textures/floor/mm_street_03.png"); got != components.BlendNone {
		t.Fatalf("expected None for floor texture, got %q", got)
	}
}

func TestGuessFilterModeGlowIsAdditive(t *testing.T) {
	if got := GuessFilterMode("textures/effects/genericglow.png"); got != components.BlendAdditive {
		t.Fatalf("expected Additive for glow texture, got %q", got)
	}
}
