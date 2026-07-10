package client

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
)

type cascTracker struct {
	mu         sync.Mutex
	buildKey   string
	product    string
	cascLoaded bool
}

func (t *cascTracker) apply(info CASCInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	prevKey := t.buildKey
	if prevKey != "" && info.BuildKey != "" && prevKey != info.BuildKey {
		runtimecache.ClearConverterRuntimeCaches()
		log.Printf("CASC build changed: %s -> %s", prevKey, info.BuildKey)
	}
	t.buildKey = info.BuildKey
	t.product = info.Build.Product
	t.cascLoaded = info.BuildName != ""
}

func (t *cascTracker) clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buildKey = ""
	t.product = ""
	t.cascLoaded = false
}

// clearIfWasLoaded clears tracker state and reports whether CASC was previously loaded.
// Polls against an unloaded server must not trigger cache clears every time.
func (t *cascTracker) clearIfWasLoaded() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.cascLoaded {
		return false
	}
	t.buildKey = ""
	t.product = ""
	t.cascLoaded = false
	return true
}

func (t *cascTracker) isClassic() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return IsClassicProduct(t.product)
}

// IsClassicProduct reports whether a loaded CASC product is a Classic variant.
func IsClassicProduct(product string) bool {
	switch product {
	case "wow_classic", "wow_classic_era", "wow_classic_ptr", "wow_classic_era_ptr":
		return true
	default:
		return false
	}
}

// StartCascMonitor polls getCascInfo and clears converter caches when the active build changes.
func StartCascMonitor(ctx context.Context, c Client) {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = c.GetCASCInfo(context.Background())
			}
		}
	}()
}

func (c *HTTPClient) onCascUnavailable() {
	if c.casc.clearIfWasLoaded() {
		runtimecache.ClearConverterRuntimeCaches()
	}
}

func (c *InProcessClient) onCascUnavailable() {
	if c.casc.clearIfWasLoaded() {
		runtimecache.ClearConverterRuntimeCaches()
	}
}
