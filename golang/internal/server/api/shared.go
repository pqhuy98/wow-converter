package api

import (
	"context"
	"log"
	"strconv"
	"strings"
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

// listfileSearchQuery rejects a non-empty search on a shared server.
// A blank search is the full catalog and is allowed everywhere.
func listfileSearchQuery(search string, isSharedHosting bool) (string, error) {
	if strings.TrimSpace(search) == "" {
		return "", nil
	}
	if err := assertDesktopOnly(isSharedHosting); err != nil {
		return "", err
	}
	return search, nil
}

// filterListfileSearch keeps entries where every word appears in the file name or the file id.
// Words split on whitespace and match case-insensitively, the same way the browse pages search.
// An empty search returns the list unchanged.
func filterListfileSearch(files []casc.ListfileEntry, search string) []casc.ListfileEntry {
	words := strings.Fields(strings.ToLower(search))
	if len(words) == 0 {
		return files
	}
	out := make([]casc.ListfileEntry, 0)
	for _, f := range files {
		id := strconv.Itoa(f.FileDataID)
		name := strings.ToLower(f.FileName)
		if listfileHasEveryWord(name, id, words) {
			out = append(out, f)
		}
	}
	return out
}

func listfileHasEveryWord(name, id string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(name, w) && !strings.Contains(id, w) {
			return false
		}
	}
	return true
}
