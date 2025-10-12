package casc

import (
	"bufio"
	"strings"
)

// ParseVersionConfig mirrors wow.export/src/js/casc/version-config.js
// Returns slice of map[string]string preserving string values.
func ParseVersionConfig(data string) []map[string]string {
	var entries []map[string]string
	lines := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return entries
	}
	headers := strings.Split(lines[0], "|")
	fields := make([]string, len(headers))
	for i := range headers {
		name := headers[i]
		if p := strings.Index(name, "!"); p >= 0 {
			name = name[:p]
		}
		fields[i] = strings.ReplaceAll(name, " ", "")
	}
	scanner := bufio.NewScanner(strings.NewReader(strings.Join(lines[1:], "\n")))
	for scanner.Scan() {
		entry := scanner.Text()
		entry = strings.TrimSpace(entry)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		node := make(map[string]string)
		parts := strings.Split(entry, "|")
		for i := 0; i < len(parts) && i < len(fields); i++ {
			node[fields[i]] = parts[i]
		}
		entries = append(entries, node)
	}
	return entries
}
