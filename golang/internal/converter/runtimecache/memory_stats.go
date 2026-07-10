package runtimecache

// MemoryStatHook returns a flat map of diagnostic counters for debugMemory.
type MemoryStatHook func() map[string]any

var memoryStatHooks []MemoryStatHook

// RegisterMemoryStatHook adds converter-side memory counters (avoids import cycles).
func RegisterMemoryStatHook(fn MemoryStatHook) {
	memoryStatHooks = append(memoryStatHooks, fn)
}

// CollectMemoryStatHooks merges all registered converter memory stats.
func CollectMemoryStatHooks() map[string]any {
	out := map[string]any{}
	for _, fn := range memoryStatHooks {
		for k, v := range fn() {
			out[k] = v
		}
	}
	return out
}
