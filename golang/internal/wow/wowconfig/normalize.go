package wowconfig

import (
	"regexp"
	"strings"
)

var (
	drivePathPattern = regexp.MustCompile(`^([a-zA-Z]:)(.*)$`)
	backslashPattern = regexp.MustCompile(`\\+`)
)

// NormalizeInstallDirectory normalizes a local WoW install path for display and filesystem use.
func NormalizeInstallDirectory(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	if m := drivePathPattern.FindStringSubmatch(trimmed); len(m) == 3 {
		body := backslashPattern.ReplaceAllString(strings.ReplaceAll(m[2], "/", `\`), `\`)
		return m[1] + body
	}

	if strings.HasPrefix(trimmed, `\\`) || strings.HasPrefix(trimmed, "//") {
		return strings.ReplaceAll(trimmed, "/", `\`)
	}

	return backslashPattern.ReplaceAllString(trimmed, `\`)
}
