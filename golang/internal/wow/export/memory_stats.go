package export

// ProgressStoreStats returns active export progress snapshots.
func ProgressStoreStats() int {
	count := 0
	progressStore.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
