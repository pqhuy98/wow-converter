package wowhead

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Document-style headers match the compatibility client. A version-specific
// Chrome user agent can receive a verification page instead of model metadata.
var defaultBrowserHeaders = map[string]string{
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
	"Accept-Language":           "en-US,en;q=0.9",
	"Cache-Control":             "no-cache",
	"DNT":                       "1",
	"Pragma":                    "no-cache",
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
	"Upgrade-Insecure-Requests": "1",
	"User-Agent":                "Mozilla/5.0",
}

// HTTPClient wraps net/http with browser-like headers.
type HTTPClient struct {
	client *http.Client
	ctx    context.Context
}

// NewHTTPClient creates a wowhead HTTP client.
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{client: newSafeHTTPClient(), ctx: context.Background()}
}

// WithContext binds requests to an export without mutating the shared client.
func (c *HTTPClient) WithContext(ctx context.Context) *HTTPClient {
	if c == nil {
		c = NewHTTPClient()
	}
	return &HTTPClient{client: c.client, ctx: ctx}
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
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
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
