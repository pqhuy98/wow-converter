package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const defaultServerURL = "http://127.0.0.1:17753"

var benchFileDataIDs []uint32

func TestMain(m *testing.M) {
	baseURL := strings.TrimRight(os.Getenv("WOW_DATA_SERVER_URL"), "/")
	if baseURL == "" {
		baseURL = defaultServerURL
	}
	if ids, ok := resolveBenchIDs(baseURL); ok {
		benchFileDataIDs = ids
	}
	os.Exit(m.Run())
}

func resolveBenchIDs(baseURL string) ([]uint32, bool) {
	client := &http.Client{Timeout: 10 * time.Second}
	if !serverReady(client, baseURL) {
		return nil, false
	}

	names := []string{
		"creature/murloc/murloc.m2",
		"interface/icons/inv_misc_questionmark.blp",
		"dbfilesclient/map.db2",
	}
	var ids []uint32
	for _, name := range names {
		id, ok := lookupFileDataID(client, baseURL, name)
		if !ok {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func serverReady(client *http.Client, baseURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/rest/getCascInfo", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false
	}
	return body["id"] == "CASC_INFO"
}

func lookupFileDataID(client *http.Client, baseURL, fileName string) (uint32, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params := url.Values{"fileName": {fileName}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/rest/getFileByName?"+params.Encode(), nil)
	if err != nil {
		return 0, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, false
	}

	var body struct {
		ID         string  `json:"id"`
		FileDataID float64 `json:"fileDataID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, false
	}
	if body.ID != "LISTFILE_RESULT" || body.FileDataID == 0 {
		return 0, false
	}
	return uint32(body.FileDataID), true
}

func BenchmarkCascFileReads(b *testing.B) {
	if len(benchFileDataIDs) == 0 {
		b.Skip("wow-data-server unavailable, CASC not loaded, or listfile not ready")
	}

	baseURL := strings.TrimRight(os.Getenv("WOW_DATA_SERVER_URL"), "/")
	if baseURL == "" {
		baseURL = defaultServerURL
	}
	client := &http.Client{Timeout: 2 * time.Minute}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := benchFileDataIDs[i%len(benchFileDataIDs)]
		if err := fetchCascFile(client, baseURL, id); err != nil {
			b.Fatalf("cascFile(%d): %v", id, err)
		}
	}
}

func fetchCascFile(client *http.Client, baseURL string, fileDataID uint32) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	u := fmt.Sprintf("%s/rest/cascFile?fileDataID=%d", baseURL, fileDataID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	_, err = io.Copy(io.Discard, resp.Body)
	return err
}
