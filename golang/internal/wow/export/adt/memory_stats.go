package adt

// CacheStatsSnapshot reports in-memory ADT export caches.
type CacheStatsSnapshot struct {
	WDTEntries         int
	BakeTextureEntries int
	GameObjectMaps     int
	GameObjectRows     int
}

// CacheStats returns sizes of export-related caches.
func CacheStats() CacheStatsSnapshot {
	stats := CacheStatsSnapshot{}
	wdtCache.Range(func(_, _ any) bool {
		stats.WDTEntries++
		return true
	})
	bakeTextureCache.Range(func(_, _ any) bool {
		stats.BakeTextureEntries++
		return true
	})
	if gameObjectsByMap != nil {
		stats.GameObjectMaps = len(gameObjectsByMap)
		for _, rows := range gameObjectsByMap {
			stats.GameObjectRows += len(rows)
		}
	}
	return stats
}
