package wowhead

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

type contextCheckingTransport struct {
	t        *testing.T
	expected context.Context
}

func (r contextCheckingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Context() != r.expected {
		r.t.Error("Wowhead request lost export context")
	}
	return nil, req.Context().Err()
}

func TestExportContextCancelsWowheadRequestsAndCacheHits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &HTTPClient{client: &http.Client{Transport: contextCheckingTransport{t, ctx}}}
	bound := client.WithContext(ctx)
	cancel()
	if _, err := bound.Get("https://www.wowhead.com/npc=1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("request error = %v", err)
	}
	const cachedURL = "https://www.wowhead.com/npc=2?queue-context-test"
	respCache.Set(cachedURL, "cached")
	if _, err := FetchWithCache(bound, cachedURL); !errors.Is(err, context.Canceled) {
		t.Fatalf("cached fetch error = %v", err)
	}
	if client.ctx != nil {
		t.Fatal("WithContext mutated the original HTTP client")
	}
}
