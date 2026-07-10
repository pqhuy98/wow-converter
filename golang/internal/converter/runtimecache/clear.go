package runtimecache

import (
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db/caches"
	adtexport "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
)

var (
	converterClearHooks    []func()
	wowDataServerClearHooks []func()
)

// RegisterConverterClearHook adds a converter-side cache reset (avoids import cycles).
func RegisterConverterClearHook(fn func()) {
	converterClearHooks = append(converterClearHooks, fn)
}

// RegisterWowDataServerClearHook adds a wow-data-server cache reset (avoids import cycles).
func RegisterWowDataServerClearHook(fn func()) {
	wowDataServerClearHooks = append(wowDataServerClearHooks, fn)
}

// ClearConverterRuntimeCaches drops in-memory converter caches after CASC unload or build change.
func ClearConverterRuntimeCaches() {
	texturesource.Clear()
	for _, fn := range converterClearHooks {
		fn()
	}
}

// ClearWowDataServerRuntimeCaches drops wow-data-server caches after CASC unload.
func ClearWowDataServerRuntimeCaches() {
	caches.ResetDBCaches()
	adtexport.ReleaseAdtExportBatchMemory()
	archivecasc.DefaultCDNResolver.ClearCache()
	for _, fn := range wowDataServerClearHooks {
		fn()
	}
}
