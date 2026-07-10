package stringsort

import "testing"

func TestLessMatchesLocaleCompareCaseSensitivity(t *testing.T) {
	// localeCompare puts lowercase before uppercase at the same prefix.
	if !Less("creature/a", "creature/A") {
		t.Fatal("expected lowercase path segment to sort before uppercase")
	}
	if Less("creature/A", "creature/a") {
		t.Fatal("expected uppercase not to sort before lowercase")
	}
}

func TestLessMatchesLocaleComparePunctuation(t *testing.T) {
	// localeCompare sorts '_' before '.' at the same prefix (pristine before base).
	pristine := "world/wmo/argus/arguszone/7arg_argus_xenodarexterior01_pristine.wmo"
	base := "world/wmo/argus/arguszone/7arg_argus_xenodarexterior01.wmo"
	if !Less(pristine, base) {
		t.Fatalf("expected pristine variant before base: %q vs %q", pristine, base)
	}
	if Less(base, pristine) {
		t.Fatalf("expected base variant after pristine: %q vs %q", base, pristine)
	}
}

func TestSortConcurrent(t *testing.T) {
	left := []string{"creature/b", "creature/a", "creature/c"}
	right := []string{"textures/z", "textures/m", "textures/a"}
	done := make(chan struct{}, 2)
	go func() {
		Sort(left)
		done <- struct{}{}
	}()
	go func() {
		Sort(right)
		done <- struct{}{}
	}()
	<-done
	<-done
	if left[0] != "creature/a" || right[0] != "textures/a" {
		t.Fatalf("unexpected sort order: left=%v right=%v", left, right)
	}
}
