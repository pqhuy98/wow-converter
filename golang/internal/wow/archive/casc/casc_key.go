package casc

import (
	"encoding/hex"
	"regexp"
	"strings"
)

// CascKey is a compact in-memory representation of CASC content/encoding keys.
type CascKey string

var hexKeyPattern = regexp.MustCompile(`^[0-9a-f]+$`)

// CascKeyFromHex converts a CDN/config hex key to compact storage form.
func CascKeyFromHex(hexKey string) CascKey {
	data, _ := hex.DecodeString(hexKey)
	return CascKey(string(data))
}

// CascKeyToHex converts a compact key back to lowercase hex.
func CascKeyToHex(key CascKey) string {
	return hex.EncodeToString([]byte(key))
}

// AsCascKey accepts hex (from config) or an already-compact key.
func AsCascKey(key string) CascKey {
	if len(key) == 32 && hexKeyPattern.MatchString(key) {
		return CascKeyFromHex(key)
	}
	return CascKey(key)
}

// NormalizeHexKey lowercases a hex key for comparisons.
func NormalizeHexKey(key string) string {
	return strings.ToLower(key)
}
