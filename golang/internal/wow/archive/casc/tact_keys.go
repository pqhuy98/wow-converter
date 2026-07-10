package casc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wow/config"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/formats"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

var (
	keyRing  = map[string]string{}
	keyMu    sync.RWMutex
	isSaving bool
	saveMu   sync.Mutex
)

// GetKey retrieves a registered decryption key.
func GetKey(keyName string) (string, bool) {
	keyMu.RLock()
	defer keyMu.RUnlock()
	key, ok := keyRing[strings.ToLower(keyName)]
	return key, ok
}

func validateKeyPair(keyName, key string) bool {
	return len(keyName) == 16 && len(key) == 32
}

// AddKey adds a decryption key subject to validation.
func AddKey(keyName, key string) bool {
	if !validateKeyPair(keyName, key) {
		return false
	}
	normalizedName := strings.ToLower(keyName)
	normalizedKey := strings.ToLower(key)
	keyMu.Lock()
	defer keyMu.Unlock()
	if keyRing[normalizedName] != normalizedKey {
		keyRing[normalizedName] = normalizedKey
		log.Write("Registered new decryption key %s -> %s", normalizedName, normalizedKey)
		scheduleSave()
	}
	return true
}

// LoadTactKeys loads tact keys from disk cache and remote servers.
func LoadTactKeys() error {
	if data, err := os.ReadFile(constants.Cache.TactKeys); err == nil {
		var tactKeys map[string]string
		if json.Unmarshal(data, &tactKeys) == nil {
			added := 0
			keyMu.Lock()
			for keyName, key := range tactKeys {
				if validateKeyPair(keyName, key) {
					keyRing[strings.ToLower(keyName)] = strings.ToLower(key)
					added++
				} else {
					log.Write("Skipping invalid tact key from cache: %s -> %s", keyName, key)
				}
			}
			keyMu.Unlock()
			log.Write("Loaded %d tact keys from local cache.", added)
		}
	}

	remoteURLs := []string{config.WowConfig.TactKeysURL, config.WowConfig.TactKeysFallbackURL}
	for _, url := range remoteURLs {
		if url == "" {
			continue
		}
		res, err := formats.Get(url)
		if err != nil {
			log.Write("Failed to load tact keys from %s: %v", url, err)
			continue
		}
		if !res.OK {
			log.Write("Unable to update tactKeys from %s, HTTP %d", url, res.Status)
			continue
		}
		remoteAdded := mergeTactKeyLines(string(res.Body))
		if remoteAdded > 0 {
			log.Write("Added %d tact keys from %s", remoteAdded, url)
			scheduleSave()
		}
	}
	return nil
}

func mergeTactKeyLines(body string) int {
	lines := strings.FieldsFunc(body, func(r rune) bool { return r == '\r' || r == '\n' })
	remoteAdded := 0
	keyMu.Lock()
	for _, line := range lines {
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			continue
		}
		keyName := strings.TrimSpace(parts[0])
		key := strings.TrimSpace(parts[1])
		if validateKeyPair(keyName, key) {
			normalizedName := strings.ToLower(keyName)
			normalizedKey := strings.ToLower(key)
			if keyRing[normalizedName] != normalizedKey {
				keyRing[normalizedName] = normalizedKey
				remoteAdded++
			}
		} else {
			log.Write("Skipping invalid remote tact key: %s -> %s", keyName, key)
		}
	}
	keyMu.Unlock()
	return remoteAdded
}

func scheduleSave() {
	saveMu.Lock()
	defer saveMu.Unlock()
	if !isSaving {
		isSaving = true
		go func() {
			doSave()
			saveMu.Lock()
			isSaving = false
			saveMu.Unlock()
		}()
	}
}

func doSave() {
	keyMu.RLock()
	snapshot := make(map[string]string, len(keyRing))
	for k, v := range keyRing {
		snapshot[k] = v
	}
	keyMu.RUnlock()
	_ = os.MkdirAll(filepath.Dir(constants.Cache.TactKeys), 0o755)
	data, _ := json.MarshalIndent(snapshot, "", "\t")
	_ = os.WriteFile(constants.Cache.TactKeys, data, 0o644)
}
