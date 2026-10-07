package reportshot

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWaitDevToolsUsesPortFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	dir := t.TempDir()
	go func() {
		time.Sleep(40 * time.Millisecond)
		if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte("12345\n/devtools/browser/abc\n"), 0o644); err != nil {
			t.Error(err)
		}
	}()
	address, err := waitDevTools(ctx, dir, bytes.NewReader(nil))
	if err != nil || address != "127.0.0.1:12345" {
		t.Fatalf("got %q %v", address, err)
	}
}

func TestWaitDevToolsUsesListeningLine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	address, err := waitDevTools(ctx, t.TempDir(), strings.NewReader("DevTools listening on ws://127.0.0.1:60926/devtools/browser/abc\n"))
	if err != nil || address != "127.0.0.1:60926" {
		t.Fatalf("got %q %v", address, err)
	}
}

func TestReadDevToolsAddressIgnoresIncompletePortFile(t *testing.T) {
	dir := t.TempDir()
	if got := readDevToolsAddress(dir); got != "" {
		t.Fatalf("missing file: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte("not-a-port\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readDevToolsAddress(dir); got != "" {
		t.Fatalf("invalid port: %q", got)
	}
}

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
