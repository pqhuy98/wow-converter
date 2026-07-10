package casc

// CascMemoryStats reports in-memory CASC index sizes.
type CascMemoryStats struct {
	Loaded        bool
	IsRemote      bool
	RootEntries   int
	RootTypes     int
	EncodingEntries int
	LocalIndexes  int
}

// MemoryStats returns CASC index sizes when c is non-nil.
func MemoryStats(c CASC) CascMemoryStats {
	if c == nil {
		return CascMemoryStats{}
	}
	stats := CascMemoryStats{
		Loaded:        c.IsLoaded(),
		IsRemote:      c.IsRemote(),
		RootEntries:   len(c.RootEntries()),
		RootTypes:     len(c.RootTypes()),
		EncodingEntries: c.EncodingEntryCount(),
	}
	if local, ok := c.(*CASCLocal); ok {
		stats.LocalIndexes = len(local.LocalIndexes)
	}
	return stats
}
