package casc

import (
	"bytes"
	"errors"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var legacyPreloadedNameLookup = map[string]int{}

func parseMasterListfile(raw []byte) {
	_ = parseMasterListfileReader(bytes.NewReader(raw), nil)
}

type failedListfileReader struct{}

func (failedListfileReader) Read([]byte) (int, error) { return 0, errors.New("listfile read failed") }

func TestListfileReaderFiltersBeforeSavingNames(t *testing.T) {
	resetMasterListfileMaps()
	t.Cleanup(resetMasterListfileMaps)
	reader := strings.NewReader("1;OLD.m2\r\n2;other-build.blp\r1;CURRENT.m2\n3;third.wmo")
	if err := parseMasterListfileReader(reader, map[int]RootEntry{1: nil, 3: nil}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(preloadedIDLookup, map[int]string{1: "current.m2", 3: "third.wmo"}) {
		t.Fatal("selected-build mapping changed")
	}
	if err := parseMasterListfileReader(io.MultiReader(strings.NewReader("1;a\n"), failedListfileReader{}), nil); err == nil {
		t.Fatal("ignored reader error")
	}
}

func legacyMasterListfile(raw []byte) {
	for _, line := range regexp.MustCompile(`\r\n|\n|\r`).Split(string(raw), -1) {
		if line == "" {
			continue
		}
		tokens := strings.Split(line, ";")
		if len(tokens) != 2 {
			continue
		}
		id, err := strconv.Atoi(tokens[0])
		if err != nil {
			continue
		}
		name := strings.ToLower(tokens[1])
		preloadedIDLookup[id] = name
		legacyPreloadedNameLookup[name] = id
	}
}

func resetMasterListfileMaps() {
	preloadedIDLookup = map[int]string{}
	legacyPreloadedNameLookup = map[string]int{}
}

func TestMasterListfileStreamingParity(t *testing.T) {
	raw := []byte("1;ABC.m2\r\n2;texture.blp\r3;last.wmo\n\n4;extra;field\ninvalid;x\n2;replacement.blp")
	resetMasterListfileMaps()
	legacyMasterListfile(raw)
	wantID := preloadedIDLookup
	resetMasterListfileMaps()
	parseMasterListfile(raw)
	if !reflect.DeepEqual(wantID, preloadedIDLookup) {
		t.Fatal("listfile mapping changed")
	}
	resetMasterListfileMaps()
}

func TestApplyPreloadBuildsReverseLookupAfterFiltering(t *testing.T) {
	oldID, oldName, oldPreID := idLookup, nameLookup, preloadedIDLookup
	oldLoaded, oldPreloaded := listfileLoaded, isPreloaded
	defer func() {
		idLookup, nameLookup, preloadedIDLookup = oldID, oldName, oldPreID
		listfileLoaded, isPreloaded = oldLoaded, oldPreloaded
	}()
	resetMasterListfileMaps()
	parseMasterListfile([]byte("1;OLD.m2\n2;other-build.blp\n1;CURRENT.m2\n3;third.wmo"))
	isPreloaded = true
	if n := ApplyPreload(map[int]RootEntry{1: nil, 3: nil}); n != 2 {
		t.Fatal(n)
	}
	if idLookup[1] != "current.m2" || nameLookup["current.m2"] != 1 || nameLookup["third.wmo"] != 3 {
		t.Fatal("selected-build lookup changed")
	}
	if _, ok := nameLookup["old.m2"]; ok {
		t.Fatal("stale duplicate alias")
	}
	if _, ok := nameLookup["other-build.blp"]; ok {
		t.Fatal("other build leaked")
	}
}

func TestRealMasterListfileParity(t *testing.T) {
	path := os.Getenv("CASC_LISTFILE_BENCH_FILE")
	if path == "" {
		t.Skip("set CASC_LISTFILE_BENCH_FILE")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	resetMasterListfileMaps()
	legacyMasterListfile(raw)
	wantID := preloadedIDLookup
	resetMasterListfileMaps()
	parseMasterListfile(raw)
	if !reflect.DeepEqual(wantID, preloadedIDLookup) {
		t.Fatal("real listfile mapping changed")
	}
	t.Logf("matched %d file ID/name pairs", len(wantID))
	resetMasterListfileMaps()
}

func BenchmarkRealMasterListfile(b *testing.B) {
	path := os.Getenv("CASC_LISTFILE_BENCH_FILE")
	if path == "" {
		b.Skip("set CASC_LISTFILE_BENCH_FILE")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		parse func([]byte)
	}{{"legacy", legacyMasterListfile}, {"streaming", parseMasterListfile}} {
		b.Run(test.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				resetMasterListfileMaps()
				test.parse(raw)
			}
			b.ReportMetric(float64(len(preloadedIDLookup)), "entries")
		})
	}
	resetMasterListfileMaps()
}
