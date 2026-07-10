package wowhead

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

var allowedFetchHosts = map[string]struct{}{
	"wowhead.com":     {},
	"www.wowhead.com": {},
	"wow.zamimg.com":  {},
	"nether.wowhead.com": {},
}

// ValidateFetchURL ensures a URL is safe to fetch from the server (HTTPS wowhead/zamimg only).
func ValidateFetchURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL")
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme")
	}
	if u.User != nil {
		return nil, fmt.Errorf("invalid URL")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("invalid URL host")
	}
	if _, ok := allowedFetchHosts[host]; !ok {
		if !strings.HasSuffix(host, ".wowhead.com") {
			return nil, fmt.Errorf("URL host not allowed")
		}
	}
	if err := rejectLiteralIP(host); err != nil {
		return nil, err
	}
	return u, nil
}

func rejectLiteralIP(host string) error {
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("URL host not allowed")
	}
	return nil
}
