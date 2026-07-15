package api

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
)

func registerDebugMemory(r Router) {
	r.Get("/debugMemory", handleDebugMemory)
}

func handleDebugMemory(w http.ResponseWriter, _ *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	texEntries, texPngBytes := texturesource.CacheStats()
	conv := runtimecache.CollectMemoryStatHooks()
	sendJSON(w, http.StatusOK, map[string]any{
		"id": "DEBUG_MEMORY",
		"summary": formatConverterMemorySummary(m, texEntries, texPngBytes, conv),
		"process": map[string]int64{
			"heapAlloc":    int64(m.HeapAlloc),
			"heapInuse":    int64(m.HeapInuse),
			"heapIdle":     int64(m.HeapIdle),
			"heapReleased": int64(m.HeapReleased),
			"stackInuse":   int64(m.StackInuse),
			"stackSys":     int64(m.StackSys),
			"sys":          int64(m.Sys),
			"totalAlloc":   int64(m.TotalAlloc),
			"numGC":        int64(m.NumGC),
			"goroutines":   int64(runtime.NumGoroutine()),
		},
		"converter": mergeConverterStats(conv, texEntries, texPngBytes),
	})
}

func mergeConverterStats(conv map[string]any, texEntries int, texPngBytes int64) map[string]any {
	out := map[string]any{}
	for k, v := range conv {
		out[k] = v
	}
	out["textureRegistryEntries"] = texEntries
	out["textureRegistryPngBytes"] = texPngBytes
	return out
}

func formatConverterMemorySummary(m runtime.MemStats, texEntries int, texPngBytes int64, conv map[string]any) string {
	mb := func(n uint64) string { return fmt.Sprintf("%.1f MB", float64(n)/1024/1024) }
	line := fmt.Sprintf("Go heap in-use %s | heap idle (retained) %s | heap alloc %s | stack %s | sys %s | goroutines %d",
		mb(m.HeapInuse), mb(m.HeapIdle), mb(m.HeapAlloc), mb(m.StackInuse), mb(m.Sys), runtime.NumGoroutine())
	if texEntries > 0 || texPngBytes > 0 {
		line += fmt.Sprintf("\nConverter textureRegistry=%d entries (~%s PNG bytes)", texEntries, mb(uint64(texPngBytes)))
	}
	if v, ok := conv["characterBakeEntries"]; ok {
		line += fmt.Sprintf("\nCharacter bake cache entries=%v textureSlots=%v", v, conv["characterBakeTextureSlots"])
	}
	return line
}
