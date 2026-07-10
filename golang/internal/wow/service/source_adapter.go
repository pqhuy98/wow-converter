package service

import (
	"context"
	"fmt"
	"sync"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/archive/client"
	apicasc "github.com/pqhuy98/wow-converter/internal/wow/casc"
)

type inflightFile struct {
	wg   sync.WaitGroup
	data []byte
	err  error
}

type inflightFiles struct {
	mu    sync.Mutex
	calls map[string]*inflightFile
}

func (f *inflightFiles) do(key string, load func() ([]byte, error)) ([]byte, error) {
	f.mu.Lock()
	if call, ok := f.calls[key]; ok {
		f.mu.Unlock()
		call.wg.Wait()
		return call.data, call.err
	}
	call := &inflightFile{}
	call.wg.Add(1)
	if f.calls == nil {
		f.calls = map[string]*inflightFile{}
	}
	f.calls[key] = call
	f.mu.Unlock()

	call.data, call.err = load()
	call.wg.Done()

	f.mu.Lock()
	delete(f.calls, key)
	f.mu.Unlock()
	return call.data, call.err
}

// SourceAdapter wraps archive CASC as the REST-layer Source interface.
type SourceAdapter struct {
	CASC       archivecasc.CASC
	fileFlight inflightFiles
}

func (s *SourceAdapter) IsLoaded() bool {
	return s.CASC != nil && s.CASC.IsLoaded()
}

func (s *SourceAdapter) TypeName() string {
	if s.CASC.IsRemote() {
		return "CASCRemote"
	}
	return "CASCLocal"
}

func (s *SourceAdapter) Build() apicasc.BuildInfo {
	b := s.CASC.Build()
	return apicasc.BuildInfo{
		Product:       b["Product"],
		Version:       b["Version"],
		VersionsName:  b["VersionsName"],
		Active:        b["Active"],
		Armadillo:     b["Armadillo"],
		Branch:        b["Branch"],
		BuildKey:      b["BuildKey"],
		CDNHosts:      b["CDNHosts"],
		CDNKey:        b["CDNKey"],
		CDNPath:       b["CDNPath"],
		CDNServers:    b["CDNServers"],
		IMSize:        b["IMSize"],
		InstallKey:    b["InstallKey"],
		KeyRing:       b["KeyRing"],
		LastActivated: b["LastActivated"],
		Tags:          b["Tags"],
	}
}

func (s *SourceAdapter) BuildConfig() any {
	return s.CASC.BuildConfig()
}

func (s *SourceAdapter) GetBuildName() string {
	return s.CASC.GetBuildName()
}

func (s *SourceAdapter) GetBuildKey() string {
	return s.CASC.GetBuildKey()
}

func (s *SourceAdapter) GetFile(ctx context.Context, fileDataID int) ([]byte, error) {
	return s.fetchFile(ctx, fileDataID, false)
}

func (s *SourceAdapter) GetFilePartial(ctx context.Context, fileDataID int) ([]byte, error) {
	return s.fetchFile(ctx, fileDataID, true)
}

func (s *SourceAdapter) fetchFile(_ context.Context, fileDataID int, partialDecrypt bool) ([]byte, error) {
	buildKey := s.CASC.GetBuildKey()
	if cached, _ := client.ReadRawCachedFile(buildKey, fileDataID); len(cached) > 0 {
		return cached, nil
	}
	flightKey := fmt.Sprintf("%s:%d:%t", buildKey, fileDataID, partialDecrypt)
	return s.fileFlight.do(flightKey, func() ([]byte, error) {
		if cached, _ := client.ReadRawCachedFile(buildKey, fileDataID); len(cached) > 0 {
			return cached, nil
		}

		data, err := s.CASC.GetFile(fileDataID, partialDecrypt, false, true, false, archivecasc.CascKey(""))
		if err != nil {
			return nil, err
		}
		if err := data.ProcessAllBlocks(); err != nil {
			return nil, err
		}
		raw := append([]byte(nil), data.Raw()...)
		if !partialDecrypt {
			go func() {
				_ = client.WriteRawCachedFile(buildKey, fileDataID, raw)
			}()
		}
		return raw, nil
	})
}
