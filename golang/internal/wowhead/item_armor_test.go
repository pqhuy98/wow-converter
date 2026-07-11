package wowhead

import "testing"

func TestFetchItemMetaFallsBackToNativeArmorSlot(t *testing.T) {
	http := NewHTTPClient()
	slot := int(SlotLegs)
	_, err := FetchItemMeta(http, ExpansionLive, 187158, &slot)
	if err != nil {
		t.Fatalf("expected meta for display 187158 via slot fallback, got %v", err)
	}
}

func TestItemMetaCandidatePathsPrefersRequestedSlot(t *testing.T) {
	paths := itemMetaCandidatePaths(187158, int(SlotLegs))
	if paths[0] != "meta/armor/7/187158.json" {
		t.Fatalf("expected legs path first, got %q", paths[0])
	}
	foundWaist := false
	for _, p := range paths {
		if p == "meta/armor/6/187158.json" {
			foundWaist = true
			break
		}
	}
	if !foundWaist {
		t.Fatalf("expected waist fallback path in candidates: %#v", paths)
	}
}
