package components

import "strings"
import "testing"

func TestAnimationToStringClosingBraceOnOwnLine(t *testing.T) {
	anim := &Animation{
		Interpolation: InterpLinear,
		KeyFrames:     map[int]any{0: []float64{1, 2, 3}, 100: []float64{4, 5, 6}},
		GlobalSeq:     &GlobalSequence{ID: 5},
	}
	out := AnimationToString("Translation", anim)
	lines := strings.Split(out, "\n")
	lastNonEmpty := ""
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			lastNonEmpty = strings.TrimSpace(line)
		}
	}
	if lastNonEmpty != "}" {
		t.Fatalf("expected closing brace on its own line, last non-empty line=%q\nfull output:\n%s", lastNonEmpty, out)
	}
}
