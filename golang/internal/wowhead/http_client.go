package wowhead

import (
	"fmt"
	"net/http"
	"time"
)

// defaultBrowserHeaders match src/lib/wowhead-client/http-client.ts — CloudFront/WAF
// often expect document-style Chrome navigation headers.
var defaultBrowserHeaders = map[string]string{
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	"Accept-Language":           "en-US,en;q=0.9",
	"Cache-Control":             "no-cache",
	"DNT":                       "1",
	"Pragma":                    "no-cache",
	"Sec-CH-UA":                 `"Chromium";v="146", "Not-A.Brand";v="24", "Google Chrome";v="146"`,
	"Sec-CH-UA-Mobile":          "?0",
	"Sec-CH-UA-Platform":        `"Windows"`,
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
	"Upgrade-Insecure-Requests": "1",
	"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
}

// HTTPClient wraps net/http with browser-like headers.
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a wowhead HTTP client.
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{client: newSafeHTTPClient()}
}

func newSafeHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if _, err := ValidateFetchURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}

// Get performs a GET request with default browser headers.
func (c *HTTPClient) Get(rawURL string) (*http.Response, error) {
	if _, err := ValidateFetchURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range defaultBrowserHeaders {
		req.Header.Set(k, v)
	}
	return c.client.Do(req)
}

// FetchText returns response body as string.
func (c *HTTPClient) FetchText(url string) (string, error) {
	return FetchWithCache(c, url)
}
