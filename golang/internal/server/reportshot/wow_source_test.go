package reportshot

import "testing"

func TestSourceShotRejectsInvalidOptionsBeforeOpeningBrowser(t *testing.T) {
	for _, args := range [][]string{
		{"165552", "--time", "-0.5"}, {"165552", "--time", "NaN"}, {"165552", "--time", "121"},
		{"165552", "--model", "effect.mdx", "--model-scale", "0"},
		{"165552", "--view", "left,left"}, {"165552", "--view", "unknown"},
	} {
		if err := runSourceCLI(args); err == nil {
			t.Fatalf("accepted invalid options %v", args)
		}
	}
}
