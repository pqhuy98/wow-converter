package wowhead

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/cache"
)

var respCache = cache.NewLRU(256)

// FetchWithCache fetches a URL with an in-memory LRU cache.
func FetchWithCache(client *HTTPClient, url string) (string, error) {
	if client.ctx != nil && client.ctx.Err() != nil {
		return "", client.ctx.Err()
	}
	if v, ok := respCache.Get(url); ok && !isVerificationPage(v) {
		return v, nil
	}
	res, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("failed to fetch URL: %s %s", res.Status, string(body))
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	text := string(body)
	if isVerificationPage(text) {
		return "", fmt.Errorf("Wowhead requires JavaScript verification for %s; retry when the page is available", url)
	}
	respCache.Set(url, text)
	return text, nil
}

func isVerificationPage(text string) bool {
	return strings.Contains(text, "verify that you're not a robot") && strings.Contains(text, "JavaScript")
}

var gathererRe = regexp.MustCompile(`WH\.Gatherer\.addData\([^,]+,[^,]+,\s*(\{[\s\S]*?\})\);?`)

// ParseDisplayIDFromGathererHTML extracts displayId from wowhead gatherer HTML.
func ParseDisplayIDFromGathererHTML(html string, entityID int) (int, error) {
	match := gathererRe.FindStringSubmatch(html)
	if len(match) < 2 {
		return 0, fmt.Errorf("gatherer data not found")
	}
	idRe := regexp.MustCompile(fmt.Sprintf(`"%d"\s*:\s*\{[\s\S]*?"displayid"\s*:\s*(\d+)`, entityID))
	m := idRe.FindStringSubmatch(match[1])
	if len(m) < 2 {
		return 0, fmt.Errorf("displayid not found for entity %d", entityID)
	}
	var id int
	_, err := fmt.Sscanf(m[1], "%d", &id)
	return id, err
}
