package adt

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

func TestReleaseAdtExportBatchMemoryClearsCaches(t *testing.T) {
	wdtCache.Store("northrend", struct{}{})
	bakeTextureCache.Store(uint32(1), struct{}{})
	gameObjectsByMap = map[uint32]map[uint32]db.DB2Row{
		571: {1: db.DB2Row{"ID": uint32(1)}},
	}

	ReleaseAdtExportBatchMemory()

	stats := CacheStats()
	if stats.WDTEntries != 0 {
		t.Fatalf("WDTEntries = %d, want 0", stats.WDTEntries)
	}
	if stats.BakeTextureEntries != 0 {
		t.Fatalf("BakeTextureEntries = %d, want 0", stats.BakeTextureEntries)
	}
	if stats.GameObjectMaps != 0 || stats.GameObjectRows != 0 {
		t.Fatalf("game object cache not cleared: maps=%d rows=%d", stats.GameObjectMaps, stats.GameObjectRows)
	}
}
