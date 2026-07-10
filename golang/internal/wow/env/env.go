package env

import (
	"os"
	"strings"
)

func cascValue(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	v = strings.Trim(v, `"`)
	// .env files often use JSON-style escaping (D:\\Games\\WoW); Go's loader does not unescape.
	v = strings.ReplaceAll(v, `\\`, `\`)
	return v
}

// CascLocalWow returns CASC_LOCAL_WOW install directory.
func CascLocalWow() string {
	return cascValue("CASC_LOCAL_WOW")
}

// CascLocalProduct returns CASC_LOCAL_PRODUCT (default wow).
func CascLocalProduct() string {
	if v := cascValue("CASC_LOCAL_PRODUCT"); v != "" {
		return v
	}
	return "wow"
}

// CascRemoteRegion returns CASC_REMOTE_REGION.
func CascRemoteRegion() string {
	return cascValue("CASC_REMOTE_REGION")
}

// CascRemoteProduct returns CASC_REMOTE_PRODUCT (default wow).
func CascRemoteProduct() string {
	if v := cascValue("CASC_REMOTE_PRODUCT"); v != "" {
		return v
	}
	return "wow"
}
