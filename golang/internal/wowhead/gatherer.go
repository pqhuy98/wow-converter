package wowhead

import (
	"fmt"
	"io"
	"regexp"

	"github.com/pqhuy98/wow-converter/internal/cache"
)

var respCache = cache.NewLRU(256)

// FetchWithCache fetches a URL with an in-memory LRU cache.
func FetchWithCache(client *HTTPClient, url string) (string, error) {
	if v, ok := respCache.Get(url); ok {
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
	respCache.Set(url, text)
	return text, nil
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
