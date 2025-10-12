package casc

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Local struct {
	dir         string
	dataDir     string
	storageDir  string
	builds      []Build
	loaded      bool
	selectedIdx int
	indexes     map[string]indexEntry
	cache       *BuildCache
	remote      *Remote
	// build/CDN config
	buildConfig map[string]string
	cdnConfig   map[string]string
	// encoding/root state
	encodingSizes map[string]int64
	encodingKeys  map[string]string
	rootTypes     []rootType
	rootEntries   map[uint32]map[int]string
	// locale selection (default enUS)
	locale uint32
}

func NewLocal(installDir string) *Local {
	return &Local{
		dir:           installDir,
		dataDir:       filepath.Join(installDir, "Data"),
		storageDir:    filepath.Join(installDir, "Data", "data"),
		selectedIdx:   -1,
		indexes:       make(map[string]indexEntry),
		encodingSizes: make(map[string]int64),
		encodingKeys:  make(map[string]string),
		rootEntries:   make(map[uint32]map[int]string),
		locale:        Locale_enUS,
	}
}

func (l *Local) Init() error {
	log.Printf("local:init dir=%s", l.dir)
	// parse .build.info and filter known products
	f, err := os.Open(filepath.Join(l.dir, ".build.info"))
	if err != nil {
		return fmt.Errorf("invalid install; missing .build.info")
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var headers []string
	if sc.Scan() {
		headers = strings.Split(sc.Text(), "|")
		for i := range headers {
			h := strings.Split(headers[i], "!")[0]
			headers[i] = strings.ReplaceAll(h, " ", "")
		}
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		node := map[string]string{}
		for i := range parts {
			if i < len(headers) {
				node[headers[i]] = parts[i]
			}
		}
		prod := node["Product"]
		if prod == "wow" || strings.HasPrefix(prod, "wow_") || strings.HasPrefix(prod, "wowt") || strings.HasPrefix(prod, "wowxptr") {
			l.builds = append(l.builds, Build{
				Product:  prod,
				Branch:   node["Branch"],
				Version:  node["Version"],
				BuildKey: node["BuildKey"],
				CDNKey:   node["CDNKey"],
			})
		}
	}
	log.Printf("local:init builds=%d", len(l.builds))
	return nil
}

func (l *Local) LoadBuild(idx int) error {
	if idx < 0 || idx >= len(l.builds) {
		return fmt.Errorf("invalid build index")
	}
	// init per-build cache
	l.cache = NewBuildCache(l.builds[idx].BuildKey)
	if err := l.cache.Init(); err != nil {
		return err
	}
	// parse journal indices
	log.Printf("[CASC] Loading indexes")
	if err := l.loadIndexes(); err != nil {
		return err
	}
	// Load configs
	log.Printf("[CASC] Fetching build configurations")
	if err := l.loadConfigs(idx); err != nil {
		return err
	}
	// Load encoding table
	log.Printf("[CASC] Loading encoding table")
	if err := l.loadEncoding(); err != nil {
		return err
	}
	// Load root file
	log.Printf("[CASC] Loading root file")
	if err := l.loadRoot(); err != nil {
		return err
	}
	l.loaded = true
	l.selectedIdx = idx
	log.Printf("local:load ok idx=%d", idx)
	return nil
}

func (l *Local) ListBuilds() []Build { return l.builds }
func (l *Local) IsLoaded() bool      { return l.loaded }
func (l *Local) GetBuildKey() string {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.builds) {
		return ""
	}
	return l.builds[l.selectedIdx].BuildKey
}
func (l *Local) GetBuildName() string {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.builds) {
		return ""
	}
	return l.builds[l.selectedIdx].Version
}

func (l *Local) GetSelectedBuild() Build {
	if l.selectedIdx < 0 || l.selectedIdx >= len(l.builds) {
		return Build{}
	}
	return l.builds[l.selectedIdx]
}

func (l *Local) GetBuildConfigMap() map[string]string { return l.buildConfig }

// ResolveEncodingKeyByFileID returns the encoding key for a given FileDataID using loaded root/encoding.
func (l *Local) ResolveEncodingKeyByFileID(fdid uint32) (string, error) {
	entry := l.rootEntries[fdid]
	if entry == nil {
		return "", fmt.Errorf("fileDataID not found in root")
	}
	// iterate by root type index order; match current locale and exclude LowViolence
	for i := 0; i < len(l.rootTypes); i++ {
		rt := l.rootTypes[i]
		if (rt.localeFlags&l.locale) == 0 || (rt.contentFlags&ContentLowViolence) != 0 {
			continue
		}
		if ckey, ok := entry[i]; ok {
			if ekey, ok2 := l.encodingKeys[ckey]; ok2 {
				return ekey, nil
			}
		}
	}
	return "", fmt.Errorf("no encoding key for fileDataID")
}

// GetValidRootEntries returns FileDataIDs matching current locale and without LowViolence.
func (l *Local) GetValidRootEntries() []uint32 {
	out := make([]uint32, 0, len(l.rootEntries))
	for fdid, entry := range l.rootEntries {
		for i := 0; i < len(l.rootTypes); i++ {
			rt := l.rootTypes[i]
			if (rt.localeFlags&l.locale) == 0 || (rt.contentFlags&ContentLowViolence) != 0 {
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

// GetDataByEncodingKey fetches BLTE bytes for an encoding key, using local files with CDN fallback.
func (l *Local) GetDataByEncodingKey(ekey string) ([]byte, error) {
	return l.GetDataFile(ekey)
}

// --- local indexes ---

type indexEntry struct {
	index  int
	offset int
	size   int
}

func (l *Local) loadIndexes() error {
	dirEntries, err := os.ReadDir(l.storageDir)
	if err != nil {
		return err
	}
	count := 0
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".idx") {
			continue
		}
		if err := l.parseIndex(filepath.Join(l.storageDir, de.Name())); err != nil {
			log.Printf("local:index parse error %v", err)
			continue
		}
		count++
	}
	log.Printf("local:index count=%d entries=%d", count, len(l.indexes))
	return nil
}

func (l *Local) parseIndex(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	idx := NewByteBuf(b)
	headerHashSize := int(idx.ReadInt32LE())
	idx.Seek(idx.Offset() + 4 + headerHashSize)
	// align to 0x10 boundary
	next := (8 + headerHashSize + 0x0F) & 0xFFFFFFF0
	idx.Seek(next)
	dataLength := int(idx.ReadInt32LE())
	idx.Seek(idx.Offset() + 4)
	nBlocks := dataLength / 18
	for i := 0; i < nBlocks; i++ {
		key := idx.ReadHexString(9)
		// skip duplicates
		if _, ok := l.indexes[key]; ok {
			idx.Seek(idx.Offset() + 1 + 4 + 4)
			continue
		}
		idxHigh := int(idx.ReadUInt8())
		idxLow := int(idx.ReadUInt32BE())
		size := int(idx.ReadInt32LE())
		l.indexes[key] = indexEntry{index: (idxHigh << 2) | ((idxLow & 0xC0000000) >> 30), offset: idxLow & 0x3FFFFFFF, size: size}
	}
	return nil
}

// read local data, fallback to CDN if missing/corrupt
func (l *Local) GetDataFile(ckey string) ([]byte, error) {
	// attempt local
	if data, err := l.getLocalData(ckey); err == nil {
		// validate BLTE magic
		if len(data) >= 4 {
			m := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
			if m == blteMagic {
				return data, nil
			}
		}
	}
	// cache
	if cached, _ := l.cache.GetFile(ckey, "data"); cached != nil {
		return cached, nil
	}
	// remote fallback requires remote preload matching the currently selected build's product
	if l.remote == nil {
		r := NewRemote("eu")
		if err := r.Init(); err == nil {
			prod := ""
			if l.selectedIdx >= 0 && l.selectedIdx < len(l.builds) {
				prod = l.builds[l.selectedIdx].Product
			} else if len(l.builds) > 0 {
				prod = l.builds[0].Product
			}
			idx := -1
			for i, b := range r.builds {
				if b.Product == prod {
					idx = i
					break
				}
			}
			if idx >= 0 {
				if err := r.LoadBuild(idx); err == nil {
					l.remote = r
				}
			}
		}
	}
	if l.remote != nil {
		// partial from archive if available
		if arch, ok := l.remote.archives[ckey]; ok {
			url := l.remote.host + "data/" + l.remote.formatCDNKey(arch.key)
			if data, _, err := httpGetRange(url, int64(arch.offset), int64(arch.size)); err == nil {
				_ = l.cache.StoreFile(ckey, data, "data")
				return data, nil
			}
		}
		// direct fetch by encoding key
		url := l.remote.host + "data/" + l.remote.formatCDNKey(ckey)
		if data, _, err := httpGet(url); err == nil {
			_ = l.cache.StoreFile(ckey, data, "data")
			return data, nil
		}
	}
	return nil, fmt.Errorf("missing data for %s", ckey)
}

func (l *Local) getLocalData(ckey string) ([]byte, error) {
	e, ok := l.indexes[ckey[:18]]
	if !ok {
		return nil, fmt.Errorf("not in local archives")
	}
	// data.<NNN>
	path := filepath.Join(l.dataDir, "data", fmt.Sprintf("data.%03d", e.index))
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// skip BLTE header padding 0x1E bytes
	if _, err := f.Seek(int64(e.offset+0x1E), 0); err != nil {
		return nil, err
	}
	buf := make([]byte, e.size-0x1E)
	if _, err := f.Read(buf); err != nil {
		return nil, err
	}
	// ensure not zeroed
	zero := true
	for i := 0; i < len(buf); i++ {
		if buf[i] != 0 {
			zero = false
			break
		}
	}
	if zero {
		return nil, fmt.Errorf("local data empty")
	}
	return buf, nil
}

// --- configs/encoding/root ---

func (l *Local) loadConfigs(buildIdx int) error {
	// BuildConfig key is stored in .build.info BuildKey; CDNConfig needs remote fallback for most paths
	// Try local BuildConfig
	bk := l.builds[buildIdx].BuildKey
	// Local path is Data/config/<xx>/<yy>/<key>
	// BuildConfig in .build.info is actually the BuildKey; for CDNConfig, we need to derive from product
	// For parity with JS, attempt local build config by key; CDNConfig must be obtained via remote
	// Try read local BuildConfig
	// guard key length for local path segments
	if len(bk) >= 4 {
		if data, err := os.ReadFile(l.formatConfigPath(bk)); err == nil {
			l.buildConfig, err = ParseCDNConfig(string(data))
			if err != nil {
				return err
			}
		} else {
			// fallback to remote
			if err := l.ensureRemote(buildIdx); err != nil {
				return err
			}
			url := l.remote.host + "config/" + l.remote.formatCDNKey(bk)
			if b, _, err2 := httpGet(url); err2 == nil {
				cfg, err3 := ParseCDNConfig(string(b))
				if err3 != nil {
					return err3
				}
				l.buildConfig = cfg
			} else {
				return err2
			}
		}
	} else {
		// invalid key, must fallback remote
		if err := l.ensureRemote(buildIdx); err != nil {
			return err
		}
		url := l.remote.host + "config/" + l.remote.formatCDNKey(bk)
		if b, _, err2 := httpGet(url); err2 == nil {
			cfg, err3 := ParseCDNConfig(string(b))
			if err3 != nil {
				return err3
			}
			l.buildConfig = cfg
		} else {
			return err2
		}
	}
	// CDNConfig always via remote
	if err := l.ensureRemote(buildIdx); err != nil {
		return err
	}
	// Discover CDNConfig key via /versions for product/region
	// Remote already populated builds with CDNConfig; find matching
	var ckey string
	for _, b := range l.remote.builds {
		if b.Product == l.builds[buildIdx].Product {
			ckey = b.CDNConfig
			if ckey == "" {
				ckey = b.CDNKey
			}
			break
		}
	}
	if ckey == "" {
		return fmt.Errorf("missing CDNConfig key for product %s", l.builds[buildIdx].Product)
	}
	url := l.remote.host + "config/" + l.remote.formatCDNKey(ckey)
	if b, _, err := httpGet(url); err == nil {
		cfg, err2 := ParseCDNConfig(string(b))
		if err2 != nil {
			return err2
		}
		l.cdnConfig = cfg
	} else {
		return err
	}
	return nil
}

func (l *Local) loadEncoding() error {
	// buildConfig["encoding"] is of form "<size> <key>"
	encField := l.buildConfig["encoding"]
	parts := strings.Split(encField, " ")
	if len(parts) < 2 {
		return fmt.Errorf("invalid encoding field")
	}
	encKey := parts[1]
	// Prefer cached build encoding if present
	if cached, _ := l.cache.GetFile("BUILD_ENCODING", ""); len(cached) > 0 {
		log.Printf("[CASC] Encoding for build %s cached locally.", l.cache.key)
		if err := parseEncodingFile(cached, encKey, l.encodingSizes, l.encodingKeys); err != nil {
			return err
		}
		return nil
	}
	raw, err := l.GetDataFile(encKey)
	if err != nil {
		return err
	}
	_ = l.cache.StoreFile("BUILD_ENCODING", raw, "")
	if err := parseEncodingFile(raw, encKey, l.encodingSizes, l.encodingKeys); err != nil {
		return err
	}
	log.Printf("[CASC] Parsed encoding table (%d entries)", len(l.encodingKeys))
	return nil
}

func (l *Local) loadRoot() error {
	rootCKey := l.buildConfig["root"]
	// encoding table maps content key -> encoding key
	eKey, ok := l.encodingKeys[rootCKey]
	if !ok {
		return fmt.Errorf("no encoding entry for root key")
	}
	// Prefer cached build root if present
	if cached, _ := l.cache.GetFile("BUILD_ROOT", ""); len(cached) > 0 {
		n, err := parseRootFile(cached, eKey, &l.rootTypes, l.rootEntries)
		if err != nil {
			return err
		}
		log.Printf("[CASC] Parsed root file (%d entries, %d types)", n, len(l.rootTypes))
		return nil
	}
	raw, err := l.GetDataFile(eKey)
	if err != nil {
		return err
	}
	_ = l.cache.StoreFile("BUILD_ROOT", raw, "")
	l.rootTypes = nil
	l.rootEntries = make(map[uint32]map[int]string)
	n, err := parseRootFile(raw, eKey, &l.rootTypes, l.rootEntries)
	if err != nil {
		return err
	}
	log.Printf("[CASC] Parsed root file (%d entries, %d types)", n, len(l.rootTypes))
	return nil
}

func (l *Local) formatConfigPath(key string) string {
	if len(key) < 4 {
		return filepath.Join(l.dataDir, "config", key)
	}
	return filepath.Join(l.dataDir, "config", key[:2], key[2:4], key)
}

func (l *Local) ensureRemote(buildIdx int) error {
	if l.remote != nil {
		return nil
	}
	r := NewRemote("eu")
	if err := r.Init(); err != nil {
		return err
	}
	// find product
	prod := l.builds[buildIdx].Product
	sel := -1
	for i, b := range r.builds {
		if b.Product == prod {
			sel = i
			break
		}
	}
	if sel < 0 {
		return fmt.Errorf("no remote product for %s", prod)
	}
	if err := r.LoadBuild(sel); err != nil {
		return err
	}
	l.remote = r
	return nil
}
