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

func TestEndConversionExportClearsWhenLastRequestFinishes(t *testing.T) {
	conversionExportsInFlight.Store(0)
	wdtCache.Store("northrend", struct{}{})

	BeginConversionExport()
	BeginConversionExport()
	EndConversionExport()
	if stats := CacheStats(); stats.WDTEntries != 1 {
		t.Fatalf("WDTEntries = %d after first End, want 1", stats.WDTEntries)
	}

	EndConversionExport()
	if stats := CacheStats(); stats.WDTEntries != 0 {
		t.Fatalf("WDTEntries = %d after last End, want 0", stats.WDTEntries)
	}
}
