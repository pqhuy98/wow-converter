package adt

import (
	"runtime"
	"runtime/debug"
	"sync/atomic"
)

var conversionExportsInFlight atomic.Int32

// BeginConversionExport marks the start of an in-memory ADT conversion request.
func BeginConversionExport() {
	conversionExportsInFlight.Add(1)
}

// EndConversionExport drops batch caches when the last parallel conversion request finishes.
func EndConversionExport() {
	if conversionExportsInFlight.Add(-1) == 0 {
		ReleaseAdtExportBatchMemory()
		// ponytail: parallel tile exports spike heap; return idle pages to the OS so RSS falls back toward baseline.
		runtime.GC()
		debug.FreeOSMemory()
	}
}

// ReleaseAdtExportBatchMemory drops batch-scoped ADT export caches after a multi-tile run finishes.
func ReleaseAdtExportBatchMemory() {
	ClearBakeTextureCache()
	ClearExporterCache()
	ClearGameObjectsCache()
}
