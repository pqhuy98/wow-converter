package casc

import "strings"

// VersionConfigEntry is a parsed version config row.
type VersionConfigEntry map[string]string

// ParseVersionConfig parses Blizzard version config files.
func ParseVersionConfig(data string) []VersionConfigEntry {
	lines := strings.Split(data, "\n")
	if len(lines) == 0 {
		return nil
	}
	headerLine := lines[0]
	lines = lines[1:]
	headers := strings.Split(headerLine, "|")
	fields := make([]string, len(headers))
	for i, h := range headers {
		fields[i] = strings.ReplaceAll(strings.Split(h, "!")[0], " ", "")
	}
	var entries []VersionConfigEntry
	for _, entry := range lines {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		node := VersionConfigEntry{}
		entryFields := strings.Split(entry, "|")
		for i, v := range entryFields {
			if i < len(fields) {
				node[fields[i]] = v
			}
		}
		entries = append(entries, node)
	}
	return entries
}
