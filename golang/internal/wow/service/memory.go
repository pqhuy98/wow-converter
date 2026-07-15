package service

import (
	"fmt"
	"runtime"

	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	apicasc "github.com/pqhuy98/wow-converter/internal/wow/casc"
	wowchar "github.com/pqhuy98/wow-converter/internal/wow/character"
	"github.com/pqhuy98/wow-converter/internal/wow/db/caches"
	"github.com/pqhuy98/wow-converter/internal/wow/export"
	adtexport "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

// MemoryDiagnostics collects runtime memory stats for debugMemory.
type MemoryDiagnostics struct{}

func (MemoryDiagnostics) Collect() apicasc.MemoryDiagnostics {
	listfile := archivecasc.GetMemoryStats()
	indexes := archivecasc.GetIndexMemoryStats()
	creatureInit, creatureDisplays, displayIDMap := caches.GetCreatureCacheStats()
	itemDisplays := caches.GetItemDisplayCacheStats()
	modelResIDs, modelFileDataIDs := caches.GetModelFileDataCacheStats()
	textureMatIDs, textureFileDataIDs := caches.GetTextureFileDataCacheStats()
	exportCaches := adtexport.CacheStats()
	converterStats := runtimecache.CollectMemoryStatHooks()

	diag := apicasc.MemoryDiagnostics{
		Process: collectProcessMemory(),
		Casc:    collectCascMemory(),
		Listfile: map[string]any{
			"loaded":              listfile.Loaded,
			"isPreloaded":         listfile.IsPreloaded,
			"idLookup":            listfile.IDLookup,
			"nameLookup":          listfile.NameLookup,
			"preloadedIdLookup":   listfile.PreloadedIDLookup,
			"preloadedNameLookup": listfile.PreloadedNameLookup,
		},
		Indexes: map[string]any{
			"browseBuilt":    indexes.BrowseBuilt,
			"browseModels":   indexes.BrowseModels,
			"browseTextures": indexes.BrowseTextures,
			"mapTileBuilt":   indexes.MapTileBuilt,
			"mapTileEntries": indexes.MapTileEntries,
		},
		DBCaches: map[string]any{
			"modelCachesInitialized": caches.ModelCachesInitialized(),
			"creatures": map[string]any{
				"initialized":       creatureInit,
				"creatureDisplays":  creatureDisplays,
				"displayIDMap":      displayIDMap,
			},
			"items": map[string]any{
				"itemDisplays": itemDisplays,
			},
			"modelFileData": map[string]any{
				"modelResIDs":    modelResIDs,
				"modelFileDataIDs": modelFileDataIDs,
			},
			"textureFileData": map[string]any{
				"matResIDs":         textureMatIDs,
				"textureFileDataIDs": textureFileDataIDs,
			},
			"character": wowchar.CacheStats(),
		},
		ExportCaches: map[string]any{
			"activeProgressSnapshots": export.ProgressStoreStats(),
			"wdtEntries":              exportCaches.WDTEntries,
			"bakeTextureEntries":      exportCaches.BakeTextureEntries,
			"gameObjectMaps":          exportCaches.GameObjectMaps,
			"gameObjectRows":          exportCaches.GameObjectRows,
		},
		Converter: converterStats,
	}
	diag.Summary = formatMemoryDiagnostics(diag)
	return diag
}

func collectProcessMemory() map[string]int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]int64{
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
	}
}

func collectCascMemory() map[string]any {
	src := server.GlobalRuntime.GetCascOptional()
	if src == nil {
		return map[string]any{"loaded": false}
	}
	adapter, ok := src.(*SourceAdapter)
	if !ok || adapter.CASC == nil {
		return map[string]any{
			"loaded":   src.IsLoaded(),
			"typeName": "unknown",
		}
	}
	stats := archivecasc.MemoryStats(adapter.CASC)
	return map[string]any{
		"loaded":        stats.Loaded,
		"isRemote":      stats.IsRemote,
		"typeName":      adapter.TypeName(),
		"buildName":     adapter.GetBuildName(),
		"rootEntries":   stats.RootEntries,
		"rootTypes":     stats.RootTypes,
		"encodingEntries": stats.EncodingEntries,
		"localIndexes":  stats.LocalIndexes,
	}
}

func formatMemoryDiagnostics(d apicasc.MemoryDiagnostics) string {
	mb := func(n int64) string { return fmt.Sprintf("%.1f MB", float64(n)/1024/1024) }
	p := d.Process
	lines := []string{
		fmt.Sprintf("Go heap in-use %s | heap idle (retained) %s | heap alloc %s | stack %s | sys %s | goroutines %d",
			mb(p["heapInuse"]), mb(p["heapIdle"]), mb(p["heapAlloc"]), mb(p["stackInuse"]), mb(p["sys"]), p["goroutines"]),
	}
	if casc, ok := d.Casc["loaded"].(bool); ok && casc {
		lines = append(lines, fmt.Sprintf("CASC rootEntries=%v encodingEntries=%v localIndexes=%v build=%v",
			d.Casc["rootEntries"], d.Casc["encodingEntries"], d.Casc["localIndexes"], d.Casc["buildName"]))
	}
	if lf, ok := d.Listfile["idLookup"]; ok {
		lines = append(lines, fmt.Sprintf("Listfile id=%v name=%v preloadedId=%v preloadedName=%v",
			lf, d.Listfile["nameLookup"], d.Listfile["preloadedIdLookup"], d.Listfile["preloadedNameLookup"]))
	}
	if idx, ok := d.Indexes["browseModels"]; ok {
		lines = append(lines, fmt.Sprintf("Indexes browseModels=%v browseTextures=%v mapTileEntries=%v",
			idx, d.Indexes["browseTextures"], d.Indexes["mapTileEntries"]))
	}
	db := d.DBCaches
	lines = append(lines, fmt.Sprintf("DB creatureDisplays=%v itemDisplays=%v modelResIDs=%v textureMatIDs=%v characterLookups=%v",
		nestedInt(db, "creatures", "creatureDisplays"),
		nestedInt(db, "items", "itemDisplays"),
		nestedInt(db, "modelFileData", "modelResIDs"),
		nestedInt(db, "textureFileData", "matResIDs"),
		nestedBool(db, "character", "initialized"),
	))
	exp := d.ExportCaches
	lines = append(lines, fmt.Sprintf("Export wdt=%v bakeTextures=%v gameObjectRows=%v progressSnapshots=%v",
		exp["wdtEntries"], exp["bakeTextureEntries"], exp["gameObjectRows"], exp["activeProgressSnapshots"]))
	conv := d.Converter
	if pngBytes, ok := conv["textureRegistryPngBytes"].(int64); ok && pngBytes > 0 {
		lines = append(lines, fmt.Sprintf("Converter textureRegistry=%v entries (~%s PNG bytes)",
			conv["textureRegistryEntries"], mb(pngBytes)))
	}
	return lines[0] + "\n" + joinLines(lines[1:])
}

func nestedInt(db map[string]any, section, key string) int {
	sec, ok := db[section].(map[string]any)
	if !ok {
		return 0
	}
	v, ok := sec[key].(int)
	if ok {
		return v
	}
	return 0
}

func nestedBool(db map[string]any, section, key string) bool {
	sec, ok := db[section].(map[string]any)
	if !ok {
		return false
	}
	v, ok := sec[key].(bool)
	return ok && v
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for _, line := range lines[1:] {
		out += "\n" + line
	}
	return out
}
