package client

import (
	"context"
	"fmt"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNPCSelectionHTTPPreservesUnitInputsAndRadius(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/rest/resolveNpcDisplay" || q.Get("displayId") != "42" || q.Get("family") != "2" || q.Get("level") != "30" || q.Get("objectScale") != "0.75" || q.Get("selection") != "1" {
			t.Errorf("selection inputs: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{%q:%q,%q:true,%q:1.25}", "id", "NPC_DISPLAY_META", "found", "selectionRadius")
	}))
	defer server.Close()
	got, err := NewHTTPClient(server.URL).ResolveNpcSelection(context.Background(), 42, casc.NpcSelectionOptions{Family: 2, Level: 30, ObjectScale: .75})
	if err != nil || got != 1.25 {
		t.Fatalf("selection radius %g: %v", got, err)
	}
}
