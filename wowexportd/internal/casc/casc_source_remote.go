package casc

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Remote struct {
	region      string
	host        string
	hosts       []string
	builds      []Build
	loaded      bool
	selectedIdx int
	cdnConf     map[string]string
	buildConf   map[string]string
	cache       *BuildCache
	archives    map[string]archiveEntry
	archMu      sync.Mutex
	// encoding/root state
	encodingSizes map[string]int64
	encodingKeys  map[string]string
	rootTypes     []rootType
	rootEntries   map[uint32]map[int]string
	// locale selection (default enUS)
	locale uint32
}

func NewRemote(region string) *Remote { return &Remote{region: region} }

func (r *Remote) Init() error {
	log.Printf("remote:init region=%s", r.region)
	// Fetch versions for all products
	// Use wow.export constants mapping outside of scope, for now assume products list
	products := []string{"wow", "wowt", "wowxptr", "wow_beta", "wow_classic", "wow_classic_beta", "wow_classic_ptr", "wow_classic_era", "wow_classic_era_ptr"}
	r.builds = nil
	host := fmt.Sprintf(PatchHostFormat, r.region)
	for _, p := range products {
		url := host + p + PatchVersionConfig
		log.Printf("remote:versions %s", url)
		b, _, err := httpGet(url)
		if err != nil {
			log.Printf("remote:versions error %v", err)
			continue
		}
		cfg := ParseVersionConfig(string(b))
		for _, e := range cfg {
			e["Product"] = p
		}
		for _, e := range cfg {
			if e["Region"] == r.region {
				r.builds = append(r.builds, Build{
					Product:      e["Product"],
					Region:       e["Region"],
					BuildConfig:  e["BuildConfig"],
					CDNConfig:    e["CDNConfig"],
					VersionsName: e["VersionsName"],
				})
			}
		}
	}
	r.host = host
	log.Printf("remote:init builds=%d", len(r.builds))
	return nil
}

func (r *Remote) LoadBuild(idx int) error {
	if idx < 0 || idx >= len(r.builds) {
		return fmt.Errorf("invalid build index")
	}
	// Resolve CDN host via /cdns
	url := r.host + r.builds[idx].Product + PatchServerConfig
	log.Printf("remote:cdns %s", url)
	b, _, err := httpGet(url)
	if err != nil {
		log.Printf("remote:cdns error %v", err)
		return err
	}
	list := ParseVersionConfig(string(b))
	var server map[string]string
	for _, e := range list {
		if e["Name"] == r.region {
			server = e
			break
		}
	}
	if server == nil {
		return fmt.Errorf("region missing in cdns")
	}
	r.rankHosts(server["Hosts"], server["Path"])
	if len(r.hosts) == 0 {
		return fmt.Errorf("no reachable hosts")
	}
	r.host = r.hosts[0]
	// Get CDN and Build configs
	cdnKey := r.builds[idx].CDNConfig
	cfgSuffix := "config/" + cdnKey[:2] + "/" + cdnKey[2:4] + "/" + cdnKey
	log.Printf("remote:cdnconfig %s%s", r.host, cfgSuffix)
	b1, _, err := r.getWithFallback(cfgSuffix)
	if err != nil {
		log.Printf("remote:cdnconfig error %v", err)
		return err
	}
	r.cdnConf, err = ParseCDNConfig(string(b1))
	if err != nil {
		log.Printf("remote:cdnconfig parse error %v", err)
		return err
	}
	r.builds[idx].CDNKey = r.builds[idx].CDNConfig
	// Also fetch BuildConfig to mirror wow.export getCascInfo
	bk := r.builds[idx].BuildConfig
	if len(bk) >= 4 {
		buildSuffix := "config/" + bk[:2] + "/" + bk[2:4] + "/" + bk
		if b2, _, err2 := r.getWithFallback(buildSuffix); err2 == nil {
			if conf, err3 := ParseCDNConfig(string(b2)); err3 == nil {
				r.buildConf = conf
			}
		}
	}
	r.cache = NewBuildCache(r.builds[idx].BuildConfig)
	if err := r.cache.Init(); err != nil {
		return err
	}
	// Load archive indices to enable partial fetch
	if err := r.loadArchives(); err != nil {
		log.Printf("remote:archives load error %v", err)
		// non-fatal
	}
	// Load encoding and root to resolve file IDs
	if err := r.loadEncoding(); err != nil {
		return err
	}
	if err := r.loadRoot(); err != nil {
		return err
	}
	r.loaded = true
	r.selectedIdx = idx
	log.Printf("remote:load ok host=%s", r.host)
	return nil
}

func (r *Remote) ListBuilds() []Build { return r.builds }
func (r *Remote) IsLoaded() bool      { return r.loaded }
func (r *Remote) GetBuildKey() string {
	if r.selectedIdx < 0 || r.selectedIdx >= len(r.builds) {
		return ""
	}
	return r.builds[r.selectedIdx].BuildConfig
}
func (r *Remote) GetBuildName() string {
	if r.selectedIdx < 0 || r.selectedIdx >= len(r.builds) {
		return ""
	}
	return r.builds[r.selectedIdx].VersionsName
}

func (r *Remote) GetSelectedBuild() Build {
	if r.selectedIdx < 0 || r.selectedIdx >= len(r.builds) {
		return Build{}
	}
	return r.builds[r.selectedIdx]
}

func (r *Remote) GetBuildConfigMap() map[string]string { return r.buildConf }

// --- Archives ---

type archiveEntry struct {
	key    string
	size   int
	offset int
}

func (r *Remote) loadArchives() error {
	r.archives = make(map[string]archiveEntry)
	keys, ok := r.cdnConf["archives"]
	if !ok || keys == "" {
		return nil
	}
	arr := strings.Split(keys, " ")
	start := time.Now()
	var wg sync.WaitGroup
	sem := make(chan struct{}, 50) // parallelism similar to wow.export
	var cachedCount int32
	var dlCount int32
	// Note: r.archives writes happen inside scanArchiveIndex; no shared writes here

	for _, k := range arr {
		k := k
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			fromCache, err := r.parseArchiveIndex(k)
			if err != nil {
				log.Printf("remote:archive index error %v", err)
				return
			}
			if fromCache {
				atomic.AddInt32(&cachedCount, 1)
			} else {
				atomic.AddInt32(&dlCount, 1)
			}
		}()
	}
	wg.Wait()
	dur := time.Since(start)
	log.Printf("remote:archives loaded (cached=%d, downloaded=%d) in %s, entries=%d", cachedCount, dlCount, dur, len(r.archives))
	return nil
}

func (r *Remote) parseArchiveIndex(key string) (bool, error) {
	// Try cache first
	name := key + ".index"
	if b, _ := r.cache.GetFile(name, "indices"); b != nil {
		r.scanArchiveIndex(b, key)
		// log.Printf("remote:archive index cached %s", key)
		return true, nil
	}
	// Download
	suffix := "data/" + key[:2] + "/" + key[2:4] + "/" + key + ".index"
	b, _, err := r.getWithFallback(suffix)
	if err != nil {
		return false, err
	}
	_ = r.cache.StoreFile(name, b, "indices")
	r.scanArchiveIndex(b, key)
	log.Printf("remote:archive index downloaded %s", key)
	return false, nil
}

func (r *Remote) scanArchiveIndex(b []byte, key string) {
	buf := NewByteBuf(b)
	// Read count from end (-12)
	end := len(b) - 12
	if end < 0 {
		return
	}
	buf.Seek(end)
	count := int(buf.ReadInt32LE())
	if count*24 > len(b) {
		return
	}
	buf.Seek(0)
	for i := 0; i < count; i++ {
		hash := buf.ReadHexString(16)
		if hash == emptyMD5 {
			hash = buf.ReadHexString(16)
		}
		size := int(buf.ReadInt32BE())
		ofs := int(buf.ReadInt32BE())
		r.archMu.Lock()
		r.archives[hash] = archiveEntry{key: key, size: size, offset: ofs}
		r.archMu.Unlock()
	}
}

// helpers
func (r *Remote) formatCDNKey(s string) string { return s[:2] + "/" + s[2:4] + "/" + s }

// --- encoding/root and data fetch ---

func (r *Remote) loadEncoding() error {
	// buildConf["encoding"] is "<size> <key>"
	encField := r.buildConf["encoding"]
	parts := strings.Split(encField, " ")
	if len(parts) < 2 {
		return fmt.Errorf("invalid encoding field")
	}
	encKey := parts[1]
	// cache first
	if cached, _ := r.cache.GetFile("BUILD_ENCODING", ""); len(cached) > 0 {
		if r.encodingSizes == nil {
			r.encodingSizes = make(map[string]int64)
		}
		if r.encodingKeys == nil {
			r.encodingKeys = make(map[string]string)
		}
		if err := parseEncodingFile(cached, encKey, r.encodingSizes, r.encodingKeys); err != nil {
			return err
		}
		return nil
	}
	// fetch from CDN
	raw, _, err := r.getWithFallback("data/" + r.formatCDNKey(encKey))
	if err != nil {
		return err
	}
	_ = r.cache.StoreFile("BUILD_ENCODING", raw, "")
	if r.encodingSizes == nil {
		r.encodingSizes = make(map[string]int64)
	}
	if r.encodingKeys == nil {
		r.encodingKeys = make(map[string]string)
	}
	if err := parseEncodingFile(raw, encKey, r.encodingSizes, r.encodingKeys); err != nil {
		return err
	}
	log.Printf("[CASC] Parsed encoding table (remote, %d entries)", len(r.encodingKeys))
	return nil
}

func (r *Remote) loadRoot() error {
	rootCKey := r.buildConf["root"]
	eKey, ok := r.encodingKeys[rootCKey]
	if !ok {
		return fmt.Errorf("no encoding entry for root key")
	}
	if cached, _ := r.cache.GetFile("BUILD_ROOT", ""); len(cached) > 0 {
		n, err := parseRootFile(cached, eKey, &r.rootTypes, r.ensureRootEntries())
		if err != nil {
			return err
		}
		log.Printf("[CASC] Parsed root file (remote, %d entries, %d types)", n, len(r.rootTypes))
		return nil
	}
	raw, err := r.GetDataByEncodingKey(eKey)
	if err != nil {
		return err
	}
	_ = r.cache.StoreFile("BUILD_ROOT", raw, "")
	r.rootTypes = nil
	r.rootEntries = make(map[uint32]map[int]string)
	n, err := parseRootFile(raw, eKey, &r.rootTypes, r.rootEntries)
	if err != nil {
		return err
	}
	log.Printf("[CASC] Parsed root file (remote, %d entries, %d types)", n, len(r.rootTypes))
	return nil
}

func (r *Remote) ensureRootEntries() map[uint32]map[int]string {
	if r.rootEntries == nil {
		r.rootEntries = make(map[uint32]map[int]string)
	}
	return r.rootEntries
}

// ResolveEncodingKeyByFileID maps a FileDataID to an encoding key using loaded root/encoding.
func (r *Remote) ResolveEncodingKeyByFileID(fdid uint32) (string, error) {
	entry := r.rootEntries[fdid]
	if entry == nil {
		return "", fmt.Errorf("fileDataID not found in root")
	}
	for i := 0; i < len(r.rootTypes); i++ {
		rt := r.rootTypes[i]
		if (rt.localeFlags&r.locale) == 0 || (rt.contentFlags&ContentLowViolence) != 0 {
			continue
		}
		if ckey, ok := entry[i]; ok {
			if ekey, ok2 := r.encodingKeys[ckey]; ok2 {
				return ekey, nil
			}
		}
	}
	return "", fmt.Errorf("no encoding key for fileDataID")
}

// GetValidRootEntries returns FileDataIDs matching current locale and without LowViolence.
func (r *Remote) GetValidRootEntries() []uint32 {
	out := make([]uint32, 0, len(r.rootEntries))
	for fdid, entry := range r.rootEntries {
		for i := 0; i < len(r.rootTypes); i++ {
			rt := r.rootTypes[i]
			if (rt.localeFlags&r.locale) == 0 || (rt.contentFlags&ContentLowViolence) != 0 {
				continue
			}
			if _, ok := entry[i]; ok {
				out = append(out, fdid)
				break
			}
		}
	}
	return out
}

// GetDataByEncodingKey fetches BLTE bytes for an encoding key via CDN, using archive partials when available and caching.
func (r *Remote) GetDataByEncodingKey(ekey string) ([]byte, error) {
	// cache first
	if b, _ := r.cache.GetFile(ekey, "data"); b != nil {
		return b, nil
	}
	// partial from archive when known
	if arch, ok := r.archives[ekey]; ok {
		if data, _, err := r.getRangeWithFallback("data/"+r.formatCDNKey(arch.key), int64(arch.offset), int64(arch.size)); err == nil {
			_ = r.cache.StoreFile(ekey, data, "data")
			return data, nil
		}
	}
	// direct fetch by encoding key
	if data, _, err := r.getWithFallback("data/" + r.formatCDNKey(ekey)); err == nil {
		_ = r.cache.StoreFile(ekey, data, "data")
		return data, nil
	} else {
		return nil, err
	}
}

// rankHosts populates r.hosts ordered by lowest ping, includes server path and trailing slash.
func (r *Remote) rankHosts(hosts string, path string) {
	arr := strings.Split(hosts, " ")
	type hp struct {
		host string
		ping time.Duration
	}
	var list []hp
	for _, h := range arr {
		base := "https://" + h + "/"
		if d, err := (CDNResolver{}).Ping(base); err == nil {
			list = append(list, hp{host: base + path + "/", ping: d})
		}
	}
	if len(list) == 0 {
		r.hosts = nil
		return
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ping < list[j].ping })
	r.hosts = make([]string, len(list))
	for i := range list {
		r.hosts[i] = list[i].host
	}
}

func (r *Remote) getWithFallback(suffix string) ([]byte, int, error) {
	var lastErr error
	var lastCode int
	for _, h := range r.hosts {
		url := h + suffix
		if b, code, err := httpGet(url); err == nil {
			return b, code, nil
		} else {
			lastErr = err
			lastCode = code
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no hosts available")
	}
	return nil, lastCode, lastErr
}

func (r *Remote) getRangeWithFallback(suffix string, ofs, length int64) ([]byte, int, error) {
	var lastErr error
	var lastCode int
	for _, h := range r.hosts {
		url := h + suffix
		if b, code, err := httpGetRange(url, ofs, length); err == nil {
			return b, code, nil
		} else {
			lastErr = err
			lastCode = code
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no hosts available")
	}
	return nil, lastCode, lastErr
}
