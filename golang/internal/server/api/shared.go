package api

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

var (
	listFilesMu sync.Mutex
	listFiles   []casc.ListfileEntry
	listPending bool
)

func getListFiles(ctx context.Context, c client.Client) ([]casc.ListfileEntry, error) {
	listFilesMu.Lock()
	if listFiles != nil {
		files := listFiles
		listFilesMu.Unlock()
		return files, nil
	}
	if listPending {
		listFilesMu.Unlock()
		for {
			time.Sleep(50 * time.Millisecond)
			listFilesMu.Lock()
			if listFiles != nil {
				files := listFiles
				listFilesMu.Unlock()
				return files, nil
			}
			if !listPending {
				listFilesMu.Unlock()
				break
			}
			listFilesMu.Unlock()
		}
		listFilesMu.Lock()
	}
	listPending = true
	listFilesMu.Unlock()

	if err := c.WaitUntilReady(ctx); err != nil {
		listFilesMu.Lock()
		listPending = false
		listFilesMu.Unlock()
		return nil, err
	}

	start := time.Now()
	log.Printf("Loading full listfile index...")
	entries, err := c.SearchFiles(ctx, "", false)
	if err == nil {
		log.Printf("Loaded %d listfile entries in %.1fs", len(entries), time.Since(start).Seconds())
	}
	listFilesMu.Lock()
	listPending = false
	if err == nil {
		listFiles = entries
	}
	listFilesMu.Unlock()
	return entries, err
}

func resetListFileCache() {
	listFilesMu.Lock()
	listFiles = nil
	listPending = false
	listFilesMu.Unlock()
}

func init() {
	runtimecache.RegisterConverterClearHook(resetListFileCache)
}
