package api

import "testing"

import "github.com/pqhuy98/wow-converter/internal/wow/casc"

func TestListfileSearchQuerySharedHosting(t *testing.T) {
	if _, err := listfileSearchQuery("illidan", true); err == nil {
		t.Fatal("shared hosting should reject search")
	}
	if _, err := listfileSearchQuery("  ", true); err != nil {
		t.Fatal("blank search is the full list")
	}
	got, err := listfileSearchQuery("illidan", false)
	if err != nil || got != "illidan" {
		t.Fatalf("local: %q %v", got, err)
	}
}

func TestFilterListfileSearch(t *testing.T) {
	files := []casc.ListfileEntry{
		{FileDataID: 1514045, FileName: "sound/creature/illidan_stormrage/vo_70_illidan_attack_01.ogg"},
		{FileDataID: 10, FileName: "sound/music/zone/illidan.mp3"},
		{FileDataID: 99, FileName: "interface/icons/spell_fire.blp"},
	}

	got := filterListfileSearch(files, "illidan attack")
	if len(got) != 1 || got[0].FileDataID != 1514045 {
		t.Fatalf("words: got %+v", got)
	}

	got = filterListfileSearch(files, "ILLIDAN")
	if len(got) != 2 {
		t.Fatalf("case: got %d", len(got))
	}

	got = filterListfileSearch(files, "1514045")
	if len(got) != 1 || got[0].FileDataID != 1514045 {
		t.Fatalf("id: got %+v", got)
	}

	got = filterListfileSearch(files, "   ")
	if len(got) != len(files) {
		t.Fatalf("blank: got %d", len(got))
	}

	got = filterListfileSearch(files, "missing")
	if len(got) != 0 {
		t.Fatalf("none: got %d", len(got))
	}
}
