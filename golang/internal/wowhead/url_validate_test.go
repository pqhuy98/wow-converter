package wowhead

import "testing"

func TestValidateFetchURLRejectsSSRF(t *testing.T) {
	cases := []string{
		"https://www.wowhead.com@169.254.169.254/latest/npc=123",
		"http://www.wowhead.com/npc=123",
		"https://evil.com/npc=123",
		"https://127.0.0.1/npc=123",
	}
	for _, raw := range cases {
		if _, err := ValidateFetchURL(raw); err == nil {
			t.Fatalf("expected reject for %q", raw)
		}
	}
}

func TestValidateFetchURLAllowsWowhead(t *testing.T) {
	cases := []string{
		"https://www.wowhead.com/wotlk/npc=123",
		"https://nether.wowhead.com/gatherer?items=1",
		"https://wow.zamimg.com/modelviewer/live/meta/npc/1",
	}
	for _, raw := range cases {
		if _, err := ValidateFetchURL(raw); err != nil {
			t.Fatalf("expected allow for %q: %v", raw, err)
		}
	}
}
