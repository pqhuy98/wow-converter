package casc

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TACT key store with lazy initialization and optional disk/remote loading.
var tactOnce sync.Once
var tact map[string][]byte

func initTACT() {
	tact = make(map[string][]byte)
	// Try load from env path or default cache path; then optional remote URL
	path := strings.TrimSpace(os.Getenv("TACT_KEYS_PATH"))
	if path == "" {
		path = PathTACTKeys()
	}
	_ = loadTACTFromFile(path)

	if len(tact) == 0 {
		// Optional remote URL
		url := strings.TrimSpace(os.Getenv("TACT_KEYS_URL"))
		if url != "" {
			if b, _, err := httpGet(url); err == nil {
				if err2 := mergeTACTFromJSON(b); err2 == nil {
					// persist to cache
					_ = os.MkdirAll(filepath.Dir(PathTACTKeys()), 0o755)
					_ = os.WriteFile(PathTACTKeys(), b, 0o644)
				}
			}
		}
	}
}

// GetTACTKey returns key bytes (hex decoded) given an 8-byte key name (hex string), or nil if missing.
func GetTACTKey(keyName string) []byte {
	tactOnce.Do(initTACT)
	if v, ok := tact[strings.ToLower(keyName)]; ok {
		return v
	}
	return nil
}

// AddTACTKey allows injecting keys programmatically in tests or runtime.
func AddTACTKey(keyName string, keyHex string) error {
	tactOnce.Do(initTACT)
	b, err := hex.DecodeString(strings.TrimSpace(keyHex))
	if err != nil {
		return err
	}
	tact[strings.ToLower(keyName)] = b
	return nil
}

// ReloadTACTKeys clears and reloads keys from disk/remote per initTACT rules.
func ReloadTACTKeys() {
	tactOnce.Do(initTACT)
	for k := range tact {
		delete(tact, k)
	}
	initTACT()
}

func loadTACTFromFile(path string) error {
	if path == "" {
		return errors.New("empty path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// Try JSON first
	if err := mergeTACTFromJSON(b); err == nil {
		return nil
	}
	// Fallback: parse simple whitespace format: <keyName> <keyHex>
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(strings.ToLower(fields[0]), "0x")
		key := fields[1]
		if len(name) != 16 { // 8 bytes -> 16 hex chars
			continue
		}
		// Accept 16 or 32 byte keys (32 or 64 hex chars)
		if len(key) != 32 && len(key) != 64 {
			continue
		}
		_ = AddTACTKey(name, key)
	}
	return nil
}

func mergeTACTFromJSON(b []byte) error {
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	for k, v := range m {
		if err := AddTACTKey(strings.TrimPrefix(strings.ToLower(k), "0x"), v); err != nil {
			return fmt.Errorf("invalid TACT key for %s: %w", k, err)
		}
	}
	return nil
}
