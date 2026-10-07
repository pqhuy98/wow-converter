//go:build integration_tests

package directwmo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	directwmo "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/wmo"
	archiveclient "github.com/pqhuy98/wow-converter/internal/wow/archive/client"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/transport"
)

// TestProfileWMOTextureBake profiles one direct WMO conversion while reading
// assets from the already-running wow-data-server. Use go test's -cpuprofile
// and -memprofile flags to capture the converter process, not an API client.
func TestProfileWMOTextureBake(t *testing.T) {
	if os.Getenv("TEXTURE_BAKE_PROFILE_OUTPUT") == "" {
		t.Skip("opt-in profiler; set TEXTURE_BAKE_PROFILE_OUTPUT (scripts/profile-wmo-texture-bake.ps1)")
	}
	if os.Getenv("WOW_DATA_SERVER_URL") == "" {
		transport.ConfigureBundled()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	casc := client.NewHTTPClient("")
	readyCtx, stopWaiting := context.WithTimeout(ctx, 45*time.Second)
	defer stopWaiting()
	for {
		attemptCtx, stopAttempt := context.WithTimeout(readyCtx, 3*time.Second)
		err := casc.WaitUntilReady(attemptCtx)
		stopAttempt()
		if err == nil {
			break
		}
		select {
		case <-readyCtx.Done():
			t.Fatalf("existing wow-data-server did not become ready: %v", err)
		case <-time.After(time.Second):
		}
	}
	cascInfo, err := casc.GetCASCInfo(ctx)
	if err != nil {
		t.Fatalf("read active CASC build key: %v", err)
	}
	if cascInfo.BuildKey == "" {
		t.Fatal("active CASC build key is empty")
	}

	name := os.Getenv("TEXTURE_BAKE_PROFILE_WMO")
	if name == "" {
		name = `world\wmo\expansion11\troll\12tr_amani_hub03.wmo`
	}
	if !strings.HasSuffix(strings.ToLower(name), ".wmo") {
		name += ".wmo"
	}
	entry, err := casc.GetFileByName(ctx, name)
	if err != nil || entry.FileDataID <= 0 {
		entries, searchErr := casc.SearchFiles(ctx, filepath.Base(name), false)
		if searchErr == nil {
			wanted := common.NormalizeLocalModelRef(name)
			for _, candidate := range entries {
				if candidate.FileDataID > 0 && strings.EqualFold(common.NormalizeLocalModelRef(candidate.FileName), wanted) {
					entry = candidate
					break
				}
			}
		}
	}
	if entry.FileDataID <= 0 {
		t.Fatalf("resolve WMO %q: no positive file data ID (get-by-name error: %v)", name, err)
	}

	out := os.Getenv("TEXTURE_BAKE_PROFILE_OUTPUT")
	if out == "" {
		out = t.TempDir()
	}
	cfg := config.DefaultConfig()
	cfg.TextureBaking.Enabled = true
	animate := false
	cfg.TextureBaking.Animate = &animate
	cfg.ExportAssetDir = filepath.Join(out, "sources")
	heapSampler := startProfileHeapSampler(100 * time.Millisecond)
	started := time.Now()
	result, err := directwmo.ConvertWmoToMdl(ctx, cfg, profileSource{client: casc, buildKey: cascInfo.BuildKey}, directwmo.ConvertOptions{
		FileDataID: entry.FileDataID,
		FileName:   entry.FileName,
	})
	conversionElapsed := time.Since(started)
	memoryStats := heapSampler()
	if err != nil {
		t.Fatal(err)
	}
	if result.MDL == nil || len(result.MDL.Geosets) == 0 {
		t.Fatal("WMO conversion produced no geometry")
	}
	baked := 0
	for _, texture := range result.MDL.Textures {
		if strings.Contains(texture.Image, "baked/uv2/") {
			baked++
		}
	}
	if baked == 0 {
		t.Fatal("WMO conversion produced no baked textures")
	}
	metrics := profileConversionMetrics{
		FileName:             entry.FileName,
		FileDataID:           entry.FileDataID,
		ConversionDuration:   conversionElapsed.String(),
		ConversionDurationMS: conversionElapsed.Milliseconds(),
		Memory:               memoryStats,
	}
	if out := os.Getenv("TEXTURE_BAKE_PROFILE_OUTPUT"); out != "" {
		if err := writeProfileArtifacts(out, result, metrics); err != nil {
			t.Fatalf("write profiling artifacts: %v", err)
		}
	}
	t.Logf("WMO %s (file data ID %d): conversion %s, %d geosets, %d baked textures; heap start/peak/end %d/%d/%d bytes, allocated %d bytes, %d GCs across %d samples", entry.FileName, entry.FileDataID, conversionElapsed, len(result.MDL.Geosets), baked, memoryStats.HeapAllocStartBytes, memoryStats.PeakHeapAllocBytes, memoryStats.HeapAllocEndBytes, memoryStats.TotalAllocDelta, memoryStats.NumGCDelta, memoryStats.Samples)
}

type profileConversionMetrics struct {
	FileName             string             `json:"fileName"`
	FileDataID           int                `json:"fileDataId"`
	ConversionDuration   string             `json:"conversionDuration"`
	ConversionDurationMS int64              `json:"conversionDurationMs"`
	Memory               profileMemoryStats `json:"memory"`
}

func writeProfileArtifacts(out string, result directm2.ConvertResult, metrics profileConversionMetrics) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "model.mdl"), []byte(result.MDL.ToMdl()), 0o644); err != nil {
		return err
	}
	metricsJSON, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "conversion-metrics.json"), metricsJSON, 0o644); err != nil {
		return err
	}
	type textureRecord struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
		Bytes  int    `json:"bytes"`
	}
	paths := make([]string, 0, len(result.TexturePaths))
	for path := range result.TexturePaths {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	records := make([]textureRecord, 0, len(paths))
	for _, path := range paths {
		source, ok := texturesource.Get(path)
		if !ok || source.Kind != texturesource.KindPNG {
			continue
		}
		texturePath := filepath.Join(out, "textures", filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(texturePath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(texturePath, source.PNG, 0o644); err != nil {
			return err
		}
		hash := sha256.Sum256(source.PNG)
		records = append(records, textureRecord{Path: path, SHA256: hex.EncodeToString(hash[:]), Bytes: len(source.PNG)})
	}
	manifest, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "textures.json"), manifest, 0o644)
}

type profileSource struct {
	client   client.Client
	buildKey string
}

type profileMemoryStats struct {
	HeapAllocStartBytes uint64 `json:"heapAllocStartBytes"`
	PeakHeapAllocBytes  uint64 `json:"peakHeapAllocBytes"`
	HeapAllocEndBytes   uint64 `json:"heapAllocEndBytes"`
	TotalAllocDelta     uint64 `json:"totalAllocDeltaBytes"`
	NumGCDelta          uint32 `json:"numGCDelta"`
	Samples             uint64 `json:"samples"`
}

func startProfileHeapSampler(interval time.Duration) func() profileMemoryStats {
	var initial runtime.MemStats
	runtime.ReadMemStats(&initial)
	stop := make(chan struct{})
	done := make(chan profileMemoryStats, 1)
	go func() {
		peak, samples := initial.HeapAlloc, uint64(1)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var current runtime.MemStats
		for {
			select {
			case <-ticker.C:
				runtime.ReadMemStats(&current)
				peak = max(peak, current.HeapAlloc)
				samples++
			case <-stop:
				runtime.ReadMemStats(&current)
				peak = max(peak, current.HeapAlloc)
				deltaGC := current.NumGC - initial.NumGC
				done <- profileMemoryStats{
					HeapAllocStartBytes: initial.HeapAlloc,
					PeakHeapAllocBytes:  peak,
					HeapAllocEndBytes:   current.HeapAlloc,
					TotalAllocDelta:     current.TotalAlloc - initial.TotalAlloc,
					NumGCDelta:          deltaGC,
					Samples:             samples + 1,
				}
				return
			}
		}
	}()
	return func() profileMemoryStats {
		close(stop)
		return <-done
	}
}

func (s profileSource) GetRawFile(ctx context.Context, fileDataID int) ([]byte, error) {
	if cached, _ := archiveclient.ReadRawCachedFile(s.buildKey, fileDataID); len(cached) > 0 {
		return cached, nil
	}
	return s.client.DownloadCascFile(ctx, fileDataID)
}

func (s profileSource) GetFileName(ctx context.Context, fileDataID int) (string, error) {
	entry, err := s.client.GetFileByID(ctx, fileDataID)
	if err != nil {
		return "", err
	}
	return entry.FileName, nil
}

func (profileSource) GetModelSkins(context.Context, int) ([]directm2.ModelSkin, error) {
	return nil, nil
}

func (s profileSource) GetBuildKey(ctx context.Context) (string, error) {
	info, err := s.client.GetCASCInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.BuildKey, nil
}
