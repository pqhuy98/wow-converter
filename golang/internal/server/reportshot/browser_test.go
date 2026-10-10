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

// Opt in with REPORT_SHOT_BROWSER_TEST=1; needs an installed browser, no converter.
func TestBrowserTempCleanup(t *testing.T) {
	if os.Getenv("REPORT_SHOT_BROWSER_TEST") != "1" {
		t.Skip("Set REPORT_SHOT_BROWSER_TEST=1 for the installed-browser cleanup check")
	}
	parent := t.TempDir()
	for _, key := range []string{"TEMP", "TMP", "TMPDIR"} {
		t.Setenv(key, parent)
	}
	sentinel := filepath.Join(parent, "unrelated.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "cancel", "capture-error"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			b, err := openBrowser(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer b.close()
			// Force browser-owned scratch files as well as its profile into existence.
			var scratch string
			for _, env := range b.cmd.Environ() {
				if strings.HasPrefix(env, "TMP=") {
					scratch = strings.TrimPrefix(env, "TMP=")
				}
			}
			contained := scratch != parent && filepath.Dir(scratch) == parent
			if contained {
				if err := os.MkdirAll(filepath.Join(scratch, "msedge_url_fetcher_test"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "cancel":
				cancel()
			case "capture-error":
				if err := b.evaluate(ctx, "throw new Error('capture failed')", nil); err == nil {
					t.Error("expected evaluation failure")
				}
			default:
				var value int
				if err := b.evaluate(ctx, "6 * 7", &value); err != nil || value != 42 {
					t.Errorf("browser evaluation: value=%d err=%v", value, err)
				}
			}
			b.close()
			if !contained {
				t.Error("browser inherited shared temp instead of a capture-owned directory")
			}
			entries, err := os.ReadDir(parent)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != "unrelated.txt" {
				names := make([]string, len(entries))
				for i, entry := range entries {
					names[i] = entry.Name()
				}
				t.Errorf("browser left temp files: %v", names)
			}
		})
	}
	t.Run("startup-cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if b, err := openBrowser(ctx); err == nil {
			b.close()
			t.Fatal("expected cancelled startup to fail")
		}
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 1 || entries[0].Name() != "unrelated.txt" {
			t.Fatalf("cancelled startup left temp files: %v %v", entries, err)
		}
	})
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
		t.Fatalf("unrelated file changed: %q %v", data, err)
	}
}

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
