package reportshot

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBrowserAssetRequestsRequireQuietWindow(t *testing.T) {
	b := &browser{}
	requestEvent := func(method, id, url string) {
		t.Helper()
		params, err := json.Marshal(map[string]any{
			"requestId": id,
			"request":   map[string]string{"url": url},
		})
		if err != nil {
			t.Fatal(err)
		}
		b.trackAssetRequest(method, params)
	}

	requestEvent("Network.requestWillBeSent", "asset-1", "https://wowhead.com/modelviewer/character.m2")
	if len(b.assetRequests) != 1 || b.assetsReady() {
		t.Fatalf("started model-viewer request should block readiness: requests=%v", b.assetRequests)
	}
	lastAssetActivity := b.assetActivity
	requestEvent("Network.requestWillBeSent", "ad-1", "https://www.googletagmanager.com/gtm.js")
	requestEvent("Network.loadingFinished", "ad-1", "")
	if len(b.assetRequests) != 1 || !b.assetActivity.Equal(lastAssetActivity) {
		t.Fatalf("ad traffic changed model asset tracking: requests=%v activity=%v", b.assetRequests, b.assetActivity)
	}

	requestEvent("Network.loadingFinished", "asset-1", "")
	if len(b.assetRequests) != 0 || b.assetsReady() {
		t.Fatalf("completed request should wait for quiet interval: requests=%v", b.assetRequests)
	}
	b.assetActivity = time.Now().Add(-500 * time.Millisecond)
	if b.assetsReady() {
		t.Fatal("assets became ready before the 750 ms quiet interval")
	}
	b.assetActivity = time.Now().Add(-time.Second)
	if !b.assetsReady() {
		t.Fatal("completed assets did not become ready after the quiet interval")
	}

	requestEvent("Network.requestWillBeSent", "asset-2", "https://wowhead.com/modelviewer/equipment.m2")
	if b.assetsReady() {
		t.Fatal("new model-viewer request should reset readiness")
	}
	requestEvent("Network.loadingFailed", "asset-2", "")
	if len(b.assetRequests) != 0 || b.assetsReady() {
		t.Fatalf("failed request should clear pending state but restart quiet interval: requests=%v", b.assetRequests)
	}
	b.assetActivity = time.Now().Add(-time.Second)
	if !b.assetsReady() {
		t.Fatal("failed request did not leave the tracker ready after settling")
	}
}
