package formats

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

// DownloadFileToDisk streams large downloads to a temporary file, replacing the
// cached file only after success so a failed refresh leaves its fallback intact.
func DownloadFileToDisk(urls []string, out string) error {
	var lastErr error
	for _, url := range urls {
		for attempt := 1; attempt <= 4; attempt++ {
			lastErr = downloadFileToDisk(url, out)
			if lastErr == nil {
				return nil
			}
			log.Write("Failed to download %s: %s", url, lastErr)
			if !isRetryableDownloadError(lastErr) || attempt == 4 {
				break
			}
			time.Sleep(time.Duration(400*attempt*attempt) * time.Millisecond)
		}
	}
	return fmt.Errorf("all download attempts failed: %w", lastErr)
}

func downloadFileToDisk(url, out string) error {
	if err := ValidateFetchURL(url); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", constants.UserAgent)
	client := *httpClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many download redirects")
		}
		return ValidateFetchURL(req.URL.String())
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return downloadStatusError{status: resp.StatusCode}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(out), ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), out)
}
