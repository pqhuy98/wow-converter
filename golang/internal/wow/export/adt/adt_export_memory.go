package adt

// ReleaseAdtExportBatchMemory drops batch-scoped ADT export caches after a multi-tile run finishes.
// Call from finalizeExportProgress (or CASC unload), not per tile — parallel tile exports share bake/WDT caches.
func ReleaseAdtExportBatchMemory() {
	ClearBakeTextureCache()
	ClearExporterCache()
	ClearGameObjectsCache()
}
