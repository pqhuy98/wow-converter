package formats

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var errFetchURLNotAllowed = errors.New("fetch URL host not allowed")

// ValidateFetchURL ensures url is HTTPS to an allowlisted public host (no private/metadata SSRF).
func ValidateFetchURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid fetch URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("%w: only https is allowed", errFetchURLNotAllowed)
	}
	if parsed.User != nil {
		return fmt.Errorf("%w: credentials in URL", errFetchURLNotAllowed)
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%w: missing host", errFetchURLNotAllowed)
	}
	if isDisallowedFetchHost(host) {
		return fmt.Errorf("%w: %s", errFetchURLNotAllowed, host)
	}
	if !isAllowedFetchHost(host) {
		return fmt.Errorf("%w: %s", errFetchURLNotAllowed, host)
	}
	return nil
}

func isDisallowedFetchHost(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func isAllowedFetchHost(host string) bool {
	h := strings.ToLower(host)
	if isBlizzardCdnHost(h) {
		return true
	}
	if h == "github.com" || strings.HasSuffix(h, ".github.com") {
		return true
	}
	if strings.HasSuffix(h, ".githubusercontent.com") {
		return true
	}
	if h == "kruithne.net" || strings.HasSuffix(h, ".kruithne.net") {
		return true
	}
	return false
}
