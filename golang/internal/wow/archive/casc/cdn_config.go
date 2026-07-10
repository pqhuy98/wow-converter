package casc

import (
	"regexp"
	"strings"
)

// CDNConfigEntries maps normalized config keys to values.
type CDNConfigEntries map[string]string

var keyVarPattern = regexp.MustCompile(`([^\s]+)\s?=\s?(.*)`)

func normalizeCDNKey(key string) string {
	parts := strings.Split(key, "-")
	if len(parts) == 1 {
		return key
	}
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// ParseCDNConfig parses a CDN config file.
func ParseCDNConfig(data string) (CDNConfigEntries, error) {
	entries := CDNConfigEntries{}
	lines := strings.Split(data, "\n")
	hasValidHeader := len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "# ")
	if !hasValidHeader {
		return nil, errInvalidCDNConfig
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(line, "#") {
			continue
		}
		match := keyVarPattern.FindStringSubmatch(line)
		if match == nil {
			return nil, errInvalidCDNToken
		}
		entries[normalizeCDNKey(match[1])] = match[2]
	}
	return entries, nil
}
