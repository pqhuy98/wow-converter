package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/formats"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

// FileSource loads DB2/DBD bytes from CASC and local DBD cache.
type FileSource interface {
	GetBuildName() string
	GetFileByName(ctx context.Context, fileName string) ([]byte, error)
	GetFile(ctx context.Context, fileDataID int) ([]byte, error)
	GetDBD(ctx context.Context, tableName string) ([]byte, error)
	StoreDBD(tableName string, data []byte) error
}

// RuntimeFileSource uses the active CASC runtime.
type RuntimeFileSource struct{}

func (RuntimeFileSource) GetBuildName() string {
	src := server.GlobalRuntime.GetCascOptional()
	if src == nil {
		return ""
	}
	return src.GetBuildName()
}

func (RuntimeFileSource) GetFileByName(ctx context.Context, fileName string) ([]byte, error) {
	src, err := server.GlobalRuntime.GetCasc()
	if err != nil {
		return nil, err
	}
	id, ok := archivecasc.GetByFilename(fileName)
	if !ok || id == 0 {
		return nil, fmt.Errorf("file %s not found in listfile", fileName)
	}
	return src.GetFilePartial(ctx, id)
}

func (RuntimeFileSource) GetFile(ctx context.Context, fileDataID int) ([]byte, error) {
	src, err := server.GlobalRuntime.GetCasc()
	if err != nil {
		return nil, err
	}
	return src.GetFile(ctx, fileDataID)
}

func (RuntimeFileSource) GetDBD(_ context.Context, tableName string) ([]byte, error) {
	dbdName := tableName + ".dbd"
	path := filepath.Join(constants.Cache.DirDBD, dbdName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (RuntimeFileSource) StoreDBD(tableName string, data []byte) error {
	dbdName := tableName + ".dbd"
	path := filepath.Join(constants.Cache.DirDBD, dbdName)
	if err := os.MkdirAll(constants.Cache.DirDBD, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

var dbdDownloadMu sync.Mutex

func downloadDBD(tableName string) ([]byte, error) {
	cfg := server.GetConfig()
	urls := []string{
		fmt.Sprintf(cfg.DBDURL, tableName),
		fmt.Sprintf(cfg.DBDFallbackURL, tableName),
	}

	// Serialize DBD fetches so initModelCaches + charMeta do not burst GitHub.
	dbdDownloadMu.Lock()
	defer dbdDownloadMu.Unlock()

	log.Write("No cached DBD, downloading new from %s", urls[0])
	var lastErr error
	for i, u := range urls {
		if i > 0 {
			log.Write("Trying DBD fallback from %s", u)
		}
		data, err := downloadDBDFromURL(u)
		if err == nil {
			return data, nil
		}
		lastErr = err
		log.Write("DBD download failed from %s: %v", u, err)
	}
	return nil, fmt.Errorf("unable to download DBD for %s: %w", tableName, lastErr)
}

func downloadDBDFromURL(url string) ([]byte, error) {
	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := formats.Get(url)
		if err == nil && resp != nil && resp.OK && len(resp.Body) > 0 {
			return resp.Body, nil
		}
		if err != nil {
			lastErr = err
		} else if resp != nil {
			lastErr = fmt.Errorf("status code: %d", resp.Status)
		} else {
			lastErr = fmt.Errorf("empty response")
		}
		if !isDBDRetryable(lastErr) || attempt == maxAttempts {
			break
		}
		delay := dbdRetryDelayMS(attempt, lastErr)
		log.Write("DBD download retry %d/%d after %v (wait %dms)", attempt, maxAttempts, lastErr, delay)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return nil, lastErr
}

func isDBDRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "429") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "EOF")
}

func dbdRetryDelayMS(attempt int, err error) int {
	if strings.Contains(err.Error(), "429") {
		// GitHub rate limit — back off longer before retry or fallback.
		switch attempt {
		case 1:
			return 2000
		case 2:
			return 5000
		default:
			return 10000
		}
	}
	delay := 400 * attempt * attempt
	if delay > 8000 {
		return 8000
	}
	return delay
}
