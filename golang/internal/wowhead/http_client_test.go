package wowhead_test

import (
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

func TestWowheadFetchUsesBrowserHeaders(t *testing.T) {
	t.Parallel()
	client := wowhead.NewHTTPClient()
	const url = "https://www.wowhead.com/wotlk/npc=36597/the-lich-king"
	text, err := client.FetchText(url)
	if err != nil {
		t.Fatalf("FetchText: %v", err)
	}
	if !strings.Contains(text, `data-mv-display-id="`) {
		t.Fatalf("expected modelviewer display id in HTML, got %d bytes", len(text))
	}
}
