package reportshot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// Opt in against a generated snapshot review with REPORT_SHOT_REVIEW_TEST=1.
func TestSnapshotReviewLinksBrowser(t *testing.T) {
	if os.Getenv("REPORT_SHOT_REVIEW_TEST") != "1" {
		t.Skip("Set REPORT_SHOT_REVIEW_TEST=1 to inspect the generated review")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := openBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	review := &url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(workspace.ResolveRepoPath("tests/snapshot-tests/model/review.html"))}
	if _, err = b.call(ctx, "Page.navigate", map[string]any{"url": review.String()}); err != nil {
		t.Fatal(err)
	}
	var ready bool
	for !ready && ctx.Err() == nil {
		if err = b.evaluate(ctx, `document.querySelectorAll('.model-links a').length > 0`, &ready); err != nil {
			t.Fatal(err)
		}
		if !ready {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if !ready {
		t.Fatal("review links did not render")
	}
	if err = b.evaluate(ctx, `localStorage.setItem('snapshot-review-votes',JSON.stringify({old:{vote:'bad',comment:'old comment'}}));votes.old={vote:'bad',comment:'old comment'};`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = b.call(ctx, "Page.reload", nil); err != nil {
		t.Fatal(err)
	}
	var fresh bool
	for !fresh && ctx.Err() == nil {
		if err = b.evaluate(ctx, `document.readyState==='complete' && typeof votes!=='undefined' && Object.keys(votes).length===0 && localStorage.getItem('snapshot-review-votes')===null`, &fresh); err != nil {
			t.Fatal(err)
		}
		if !fresh {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if !fresh {
		t.Fatal("review retained previous votes/comments")
	}
	var links []string
	if err = b.evaluate(ctx, `(()=>{window.__reviewCard=Array.from(document.querySelectorAll('.card')).find(c=>c.querySelectorAll('.model-links a').length===3);return Array.from(window.__reviewCard.querySelectorAll('.model-links a'),a=>a.href);})()`, &links); err != nil {
		t.Fatal(err)
	}
	if len(links) != 3 {
		t.Fatalf("expected three model links, got %v", links)
	}
	t.Logf("Card links: %v", links)
	if _, err = b.call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 1000, "deviceScaleFactor": 1, "mobile": false}); err != nil {
		t.Fatal(err)
	}
	if err = b.evaluate(ctx, `window.__reviewCard.querySelector('.shot').click()`, nil); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = b.evaluate(ctx, `document.querySelectorAll('#lb-base a').length`, &count); err != nil || count != 3 {
		t.Fatalf("lightbox links=%d err=%v", count, err)
	}
	if err = b.evaluate(ctx, `document.querySelector('.close').click()`, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := b.call(ctx, "Page.captureScreenshot", map[string]any{"format": "png"})
	if err != nil {
		t.Fatal(err)
	}
	var shot struct {
		Data string `json:"data"`
	}
	if err = json.Unmarshal(raw, &shot); err != nil {
		t.Fatal(err)
	}
	png, err := base64.StdEncoding.DecodeString(shot.Data)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(workspace.ResolveRepoPath("docs/snapshot-review.png"), png, 0644); err != nil {
		t.Fatal(err)
	}
}

// Run after the snapshot exporter finishes: native viewer links select global WoW data.
func TestSnapshotReviewSourceLinksBrowser(t *testing.T) {
	if os.Getenv("REPORT_SHOT_REVIEW_SOURCE_TEST") != "1" {
		t.Skip("Set REPORT_SHOT_REVIEW_SOURCE_TEST=1 after catalog regeneration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	b, err := openBrowser(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
	review := &url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(workspace.ResolveRepoPath("tests/snapshot-tests/model/review.html"))}
	if _, err = b.call(ctx, "Page.navigate", map[string]any{"url": review.String()}); err != nil {
		t.Fatal(err)
	}
	var urls []string
	for len(urls) == 0 && ctx.Err() == nil {
		err = b.evaluate(ctx, `typeof CASES==='undefined' ? [] : ['classic/npc-36853-sindragosa','retail/ragnaros2-ragnaros2','retail/npc-102672-nythendra'].map(id=>CASES.find(c=>c.name===id)?.wowViewerUrl)`, &urls)
		if err != nil {
			t.Fatal(err)
		}
		if len(urls) == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if len(urls) != 3 {
		t.Fatal("review did not provide all source links")
	}
	for _, href := range urls {
		if href == "" {
			t.Fatal("source link missing")
		}
		if _, err = b.call(ctx, "Page.navigate", map[string]any{"url": href}); err != nil {
			t.Fatal(err)
		}
		hrefJSON, _ := json.Marshal(href)
		deadline := time.Now().Add(70 * time.Second)
		var state string
		for time.Now().Before(deadline) {
			err = b.evaluate(ctx, `(()=>{if(location.href!==`+string(hrefJSON)+`)return '';const d=document.querySelector('iframe')?.contentDocument;return d?.documentElement.dataset.viewerError || d?.documentElement.dataset.viewerReady || '';})()`, &state)
			if err != nil {
				t.Fatal(err)
			}
			if state != "" {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if state != "true" {
			t.Fatalf("source link did not load: %s state=%q", href, state)
		}
		var product string
		if err = b.evaluate(ctx, `fetch('/api/wow-config/status').then(r=>r.json()).then(s=>s.cascInfo?.build.Product || s.config?.product)`, &product); err != nil {
			t.Fatal(err)
		}
		destination, _ := url.Parse(href)
		if product != destination.Query().Get("product") {
			t.Fatalf("source product=%s for %s", product, href)
		}
		t.Logf("Loaded source link (%s): %s", product, href)
	}
}
