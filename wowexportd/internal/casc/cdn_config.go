package casc

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// CDNConfig parses key-value configs with dashed keys normalized to camelCase.
// Mirrors wow.export/src/js/casc/cdn-config.js semantics.
func ParseCDNConfig(data string) (map[string]string, error) {
	entries := make(map[string]string)
	r := bufio.NewReader(strings.NewReader(data))

	firstLine, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if !strings.HasPrefix(strings.TrimSpace(firstLine), "# ") {
		return nil, errors.New("invalid CDN config: unexpected start of config")
	}

	// Continue reading the rest including any remaining on first line leftover
	rest := firstLine
	for {
		line, err := r.ReadString('\n')
		rest += line
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
	}

	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid token in CDN config")
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		entries[normalizeCDNKey(key)] = val
	}
	return entries, nil
}

func normalizeCDNKey(key string) string {
	if !strings.Contains(key, "-") {
		return key
	}
	parts := strings.Split(key, "-")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}
