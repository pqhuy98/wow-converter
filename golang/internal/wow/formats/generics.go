// Package formats provides network and file helpers ported from generics.ts.
package formats

import (
	"bytes"
	"compress/zlib"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

var blizzardHostPattern = regexp.MustCompile(`(?i)\.(blizzard\.com|battle\.net)$`)

// HTTPResponse mirrors fetch Response fields used by the TS port.
type HTTPResponse struct {
	Status int
	Body   []byte
	OK     bool
}

type downloadStatusError struct {
	status int
}

func (e downloadStatusError) Error() string {
	return fmt.Sprintf("status code: %d", e.status)
}

var httpClient = &http.Client{
	Timeout: 10 * time.Minute,
	Transport: &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig:     &tls.Config{},
	},
}

// Get performs HTTP GET with fallback URL support.
func Get(urls ...string) (*HTTPResponse, error) {
	if len(urls) == 0 {
		return nil, errors.New("no URL provided")
	}
	index := 1
	var last *HTTPResponse
	for len(urls) > 0 {
		currentURL := urls[0]
		urls = urls[1:]
		resp, err := doGet(currentURL, -1, -1)
		if err != nil {
			return nil, err
		}
		log.Write("get -> [%d][%d] %s", index, resp.Status, currentURL)
		index++
		last = resp
		if resp.OK {
			return resp, nil
		}
	}
	return last, nil
}

func doGet(url string, partialOfs, partialLen int) (*HTTPResponse, error) {
	data, status, err := requestData(url, partialOfs, partialLen)
	if err != nil {
		return nil, err
	}
	return &HTTPResponse{
		Status: status,
		Body:   data,
		OK:     status >= 200 && status <= 299,
	}, nil
}

func isRetryableDownloadError(err error) bool {
	if err == nil {
		return false
	}
	var statusErr downloadStatusError
	if errors.As(err, &statusErr) {
		return statusErr.status == http.StatusRequestTimeout ||
			statusErr.status == http.StatusTooManyRequests ||
			statusErr.status >= http.StatusInternalServerError
	}
	msg := err.Error()
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "EOF")
}

func requestData(url string, partialOfs, partialLen int) ([]byte, int, error) {
	maxAttempts := 4
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		data, status, err := requestDataSingleHop(url, partialOfs, partialLen)
		if err == nil {
			return data, status, nil
		}
		lastErr = err
		if !isRetryableDownloadError(err) || attempt == maxAttempts {
			return nil, 0, err
		}
		delay := min(400*attempt*attempt, 8000)
		log.Write("requestData retry %d/%d after %s (wait %dms)", attempt, maxAttempts, err.Error(), delay)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return nil, 0, lastErr
}

func requestDataSingleHop(url string, partialOfs, partialLen int) ([]byte, int, error) {
	if err := ValidateFetchURL(url); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", constants.UserAgent)
	if partialOfs > -1 && partialLen > -1 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", partialOfs, partialOfs+partialLen-1))
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusFound {
		loc := resp.Header.Get("Location")
		if loc == "" {
			return nil, 0, errors.New("redirect without Location header")
		}
		log.Write("Got redirect to %s", loc)
		return requestData(resolveURL(url, loc), partialOfs, partialLen)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 302 {
		return nil, resp.StatusCode, downloadStatusError{status: resp.StatusCode}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

func resolveURL(base, loc string) string {
	if strings.HasPrefix(loc, "http://") || strings.HasPrefix(loc, "https://") {
		return loc
	}
	if strings.HasPrefix(loc, "/") {
		// best-effort join
		if i := strings.Index(base, "://"); i >= 0 {
			rest := base[i+3:]
			if j := strings.Index(rest, "/"); j >= 0 {
				return base[:i+3+j] + loc
			}
		}
	}
	return loc
}

// boundedConcurrency caps a worker limit to the number of jobs (min 1 when jobs > 0).
func boundedConcurrency(limit, jobs int) int {
	if jobs < 1 {
		return 1
	}
	if limit < 1 {
		limit = 1
	}
	if limit > jobs {
		return jobs
	}
	return limit
}

// Queue dispatches handlers with a concurrency limit.
func Queue[T any](items []T, handler func(T) error, limit int) error {
	limit = boundedConcurrency(limit, len(items))
	sem := make(chan struct{}, limit)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	for _, item := range items {
		wg.Add(1)
		go func(it T) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := handler(it); err != nil {
				select {
				case errCh <- err:
				default:
				}
			}
		}(item)
	}
	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

// BatchWork processes items in chunks, yielding between batches.
func BatchWork[T any](name string, items []T, handler func(T, int), batchSize int) {
	if batchSize <= 0 {
		batchSize = 1000
	}
	total := len(items)
	for start := 0; start < total; start += batchSize {
		end := start + batchSize
		if end > total {
			end = total
		}
		for i := start; i < end; i++ {
			handler(items[i], i)
		}
		if end < total {
			time.Sleep(0)
		}
	}
	log.Write("batchWork \"%s\" processed %d items", name, total)
}

func Filesize(bytes int) string {
	size := float64(bytes)
	if math.IsNaN(size) {
		size = 0
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	unitIndex := 0
	if size > 0 {
		unitIndex = int(math.Log(size) / math.Log(1024))
	}
	if unitIndex < 0 {
		unitIndex = 0
	}
	if unitIndex >= len(units) {
		unitIndex = len(units) - 1
	}
	if size > 0 {
		size = size / math.Pow(1024, float64(unitIndex))
	}
	return fmt.Sprintf("%.2f %s", size, units[unitIndex])
}

// Ping measures response time for a URL.
// CDN host resolution pings CDN roots (e.g. https://level3.blizzard.com/) which
// often return 403/404; only transport failures should count as errors (wow.export parity).
func Ping(url string) (int, error) {
	start := time.Now()
	if err := pingRequest(url); err != nil {
		return 0, err
	}
	return int(time.Since(start).Milliseconds()), nil
}

func pingRequest(url string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", constants.UserAgent)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusFound {
		loc := resp.Header.Get("Location")
		if loc == "" {
			return errors.New("redirect without Location header")
		}
		io.Copy(io.Discard, resp.Body)
		return pingRequest(resolveURL(url, loc))
	}

	_, err = io.Copy(io.Discard, resp.Body)
	return err
}

// ReadJSON reads a JSON file, returning nil on error.
func ReadJSON(file string, _ bool) (map[string]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil
	}
	var out map[string]string
	if err := jsonUnmarshal(data, &out); err != nil {
		return nil, nil
	}
	return out, nil
}

func jsonUnmarshal(data []byte, out any) error {
	return json.Unmarshal(data, out)
}

// DownloadFile downloads a file with optional partial range and output path.
func DownloadFile(urls []string, out string, partialOfs, partialLen int, deflate bool) (*buffer.Buffer, error) {
	for _, currentURL := range urls {
		log.Write("downloadFile -> %s", currentURL)
		data, _, err := requestData(currentURL, partialOfs, partialLen)
		if err != nil {
			log.Write("Failed to download from %s: %s", currentURL, err.Error())
			continue
		}
		if deflate {
			r, zerr := zlib.NewReader(bytes.NewReader(data))
			if zerr != nil {
				log.Write("Failed to inflate from %s: %s", currentURL, zerr.Error())
				continue
			}
			inflated, rerr := io.ReadAll(r)
			r.Close()
			if rerr != nil {
				log.Write("Failed to read inflated data from %s: %s", currentURL, rerr.Error())
				continue
			}
			data = inflated
		}
		wrapped := buffer.From(data)
		if out != "" {
			if err := CreateDirectory(filepath.Dir(out)); err != nil {
				return nil, err
			}
			if err := wrapped.WriteToFile(out); err != nil {
				return nil, err
			}
		}
		return wrapped, nil
	}
	return nil, errors.New("all download attempts failed")
}

// CreateDirectory creates all directories in a path.
func CreateDirectory(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// FileExists checks whether a file exists.
func FileExists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

// ReadFile reads a portion of a file.
func ReadFile(file string, offset, length int) (*buffer.Buffer, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := buffer.Alloc(length, false)
	_, err = f.ReadAt(buf.Raw(), int64(offset))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buf, nil
}

// DeleteDirectory recursively deletes a directory.
func DeleteDirectory(dir string) (int, error) {
	var deleteSize int
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, nil
	}
	for _, entry := range entries {
		entryPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			s, _ := DeleteDirectory(entryPath)
			deleteSize += s
		} else {
			info, err := entry.Info()
			if err == nil {
				deleteSize += int(info.Size())
			}
			_ = os.Remove(entryPath)
		}
	}
	_ = os.Remove(dir)
	return deleteSize, nil
}

func isBlizzardCdnHost(hostname string) bool {
	return blizzardHostPattern.MatchString(hostname)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
