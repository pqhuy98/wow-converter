package wowhead

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type verificationTransport struct{ calls int }

func (tr *verificationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.calls++
	text := `<h1>JavaScript is disabled</h1>In order to continue, we need to verify that you're not a robot.`
	if tr.calls > 1 {
		text = `<button data-mv-display-id="30721">View in 3D</button>`
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(text)), Header: make(http.Header), Request: req}, nil
}

func TestVerificationResponseIsNotCachedAsNPCMetadata(t *testing.T) {
	const url = "https://www.wowhead.com/wotlk/npc=36597/the-lich-king?verification-cache-test"
	transport := &verificationTransport{}
	client := &HTTPClient{client: &http.Client{Transport: transport}}
	if _, err := GetZamURLFromWowheadURL(client, url); err == nil || !strings.Contains(err.Error(), "JavaScript verification") {
		t.Fatalf("verification error=%v", err)
	}
	if _, ok := respCache.Get(url); ok {
		t.Fatal("verification response cached as metadata")
	}
	zam, err := GetZamURLFromWowheadURL(client, url)
	if err != nil || zam.DisplayID != 30721 || zam.Expansion != ExpansionWrath {
		t.Fatalf("retry=%+v, %v", zam, err)
	}
	if transport.calls != 2 {
		t.Fatalf("requests=%d", transport.calls)
	}
	if _, err := GetZamURLFromWowheadURL(client, url); err != nil || transport.calls != 2 {
		t.Fatal("successful metadata cache lost")
	}
	const staleURL = "https://www.wowhead.com/wotlk/npc=36597/the-lich-king?stale-verification-cache-test"
	respCache.Set(staleURL, "JavaScript is disabled; verify that you're not a robot")
	if zam, err := GetZamURLFromWowheadURL(client, staleURL); err != nil || zam.DisplayID != 30721 || transport.calls != 3 {
		t.Fatalf("cached verification was not retried: %+v, %v", zam, err)
	}
}
