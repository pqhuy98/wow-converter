package stringsort

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
)

func TestChunkedSortMatchesCollator(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	input := []string{"a", "A", "á", "ä", "ß", "ss", "中", "", "x_1", "x.1", "é", "e\u0301"}
	for i := 0; i < 12000; i++ {
		input = append(input, fmt.Sprintf("world/textures/zone_%d/path_%d_ä.blp", rng.Intn(700), i))
	}
	want := append([]string(nil), input...)
	NewEnglishCollator().SortStrings(want)
	Sort(input)
	if !reflect.DeepEqual(input, want) {
		t.Fatal("collation order changed")
	}
}

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

func TestLessConcurrent(t *testing.T) {
	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if !Less("skin/a", "skin/b") || Less("skin/b", "skin/a") {
					t.Error("concurrent Less disagreed with itself")
					return
				}
			}
		}()
	}
	wg.Wait()
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
