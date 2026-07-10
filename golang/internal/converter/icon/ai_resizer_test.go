package icon

import (
	"strings"
	"testing"
)

func TestModelScale(t *testing.T) {
	if got := modelScale("realesrgan-x4plus"); got != 4 {
		t.Fatalf("got %d want 4", got)
	}
	if got := modelScale("realesr-x2plus"); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
}

func TestAiScaleForTarget(t *testing.T) {
	if got := aiScaleForTarget(64, Size256); got != 4 {
		t.Fatalf("64->256 got %d want 4", got)
	}
	if got := aiScaleForTarget(128, Size256); got != 2 {
		t.Fatalf("128->256 got %d want 2", got)
	}
	if got := aiScaleForTarget(64, Size128); got != 2 {
		t.Fatalf("64->128 got %d want 2", got)
	}
}

func TestBuildUpscaleArgsIncludesScaleWhenNeeded(t *testing.T) {
	args := buildUpscaleArgs("in.png", "out.png", `C:\models\realesrgan-x4plus`, 2)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-s 2") {
		t.Fatalf("expected scale flag in args: %v", args)
	}
}
