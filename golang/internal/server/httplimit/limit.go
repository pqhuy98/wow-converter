package httplimit

const (
	// MaxRequestBodyBytes caps JSON request bodies on public API and wow-data-server REST.
	MaxRequestBodyBytes int64 = 4 << 20 // 4 MiB
)
