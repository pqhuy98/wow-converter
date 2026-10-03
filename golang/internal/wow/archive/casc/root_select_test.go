package casc

import "testing"

// Classic roots list a HighRes (0x1) and a low-res variant with the same locale; HighRes must win every time, wherever it sits.
func TestSelectRootContentKeyPrefersHighRes(t *testing.T) {
	b := &BaseCASC{
		LocaleValue: LocaleEnUS,
		RootTypesList: []RootType{
			{ContentFlags: 0x02080000, LocaleFlags: 0x173f6},
			{ContentFlags: 0x02080001, LocaleFlags: 0x173f6},
			{ContentFlags: 0x02080000, LocaleFlags: 0x173f6},
		},
	}
	for _, root := range []RootEntry{
		{{TypeIdx: 0, Key: "lowres"}, {TypeIdx: 1, Key: "highres"}},
		{{TypeIdx: 1, Key: "highres"}, {TypeIdx: 0, Key: "lowres"}},
	} {
		for i := 0; i < 200; i++ {
			if got := b.selectRootContentKey(root); got != "highres" {
				t.Fatalf("call %d picked %q, want highres", i, got)
			}
		}
	}
	if got := b.selectRootContentKey(RootEntry{{TypeIdx: 0, Key: "a"}, {TypeIdx: 2, Key: "b"}}); got != "a" {
		t.Fatalf("without HighRes picked %q, want first in root order", got)
	}
}
