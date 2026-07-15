package casc

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/formats"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/runtime"
)

type archiveEntry struct {
	Key    string
	Size   int
	Offset int
}

// CASCRemote is a remote CDN CASC source.
type CASCRemote struct {
	*BaseCASC
	Archives     map[string]archiveEntry
	archivesMu   sync.Mutex
	Region       string
	Host         string
	ServerConfig VersionConfigEntry
	cdnHosts     []string
	cdnRetryAt   map[string]time.Time
	cdnHostsMu   sync.Mutex
}

// NewCASCRemote creates a remote CASC source.
func NewCASCRemote(region string) *CASCRemote {
	return &CASCRemote{
		BaseCASC:   NewBaseCASC(true),
		Archives:   map[string]archiveEntry{},
		Region:     region,
		cdnRetryAt: map[string]time.Time{},
	}
}

// Init initializes the remote CASC source.
func (c *CASCRemote) Init() error {
	log.Write("Initializing remote CASC source (%s)", c.Region)
	c.Host = fmt.Sprintf(constants.Patch.Host, c.Region)
	c.BuildsList = nil
	for _, product := range constants.Products {
		configs, err := c.GetVersionConfig(product.Product)
		if err != nil {
			continue
		}
		for _, entry := range configs {
			if entry["Region"] == c.Region {
				c.BuildsList = append(c.BuildsList, entry)
			}
		}
	}
	log.Write("%v", c.BuildsList)
	return nil
}

// GetVersionConfig downloads the remote version config for a product.
func (c *CASCRemote) GetVersionConfig(product string) ([]VersionConfigEntry, error) {
	config, err := c.GetConfig(product, constants.Patch.VersionConfig)
	if err != nil {
		return nil, err
	}
	for i := range config {
		config[i]["Product"] = product
	}
	return config, nil
}

// GetConfig downloads and parses a version config file.
func (c *CASCRemote) GetConfig(product, file string) ([]VersionConfigEntry, error) {
	url := c.Host + product + file
	res, err := formats.Get(url)
	if err != nil {
		return nil, err
	}
	if !res.OK {
		return nil, fmt.Errorf("HTTP %d from remote CASC endpoint: %s", res.Status, url)
	}
	return ParseVersionConfig(string(res.Body)), nil
}

// GetCDNConfig downloads and parses a CDN config file.
func (c *CASCRemote) GetCDNConfig(key string, cdnHosts []string) (CDNConfigEntries, error) {
	hostsToTry := cdnHosts
	if len(hostsToTry) == 0 {
		hostsToTry = []string{c.Host}
	}
	var lastError error
	for _, host := range hostsToTry {
		url := host + "config/" + c.FormatCDNKey(key)
		log.Write("Attempting to retrieve CDN config from: %s", url)
		res, err := formats.Get(url)
		if err != nil {
			lastError = err
			DefaultCDNResolver.MarkHostFailed(host)
			continue
		}
		if !res.OK {
			lastError = fmt.Errorf("HTTP %d from CDN config endpoint", res.Status)
			DefaultCDNResolver.MarkHostFailed(host)
			continue
		}
		cfg, err := ParseCDNConfig(string(res.Body))
		if err != nil {
			lastError = err
			DefaultCDNResolver.MarkHostFailed(host)
			continue
		}
		if host != c.Host {
			log.Write("Successfully retrieved config from fallback host: %s", host)
			c.Host = host
		}
		return cfg, nil
	}
	msg := "unknown error"
	if lastError != nil {
		msg = lastError.Error()
	}
	return nil, fmt.Errorf("unable to retrieve CDN config file %s from any CDN host. Last error: %s", key, msg)
}

// GetFile obtains a file by fileDataID.
func (c *CASCRemote) GetFile(fileDataID int, partialDecrypt, suppressLog, _, _ bool, contentKey CascKey) (*BLTEReader, error) {
	if !suppressLog {
		name, _ := GetByID(fileDataID)
		log.Write("Loading remote CASC file %d (%s)", fileDataID, name)
	}
	var encodingKey CascKey
	var err error
	if contentKey != "" {
		encodingKey, err = c.GetEncodingKeyForContentKey(contentKey)
	} else {
		encodingKey, err = c.GetEncodingKey(fileDataID)
	}
	if err != nil {
		return nil, err
	}
	encodingKeyHex := CascKeyToHex(encodingKey)
	data, _ := c.CacheStore.GetFile(encodingKeyHex, constants.Cache.DirData)
	if data == nil {
		archive, ok := c.Archives[encodingKeyHex]
		if ok {
			data, err = c.GetDataFilePartial(c.FormatCDNKey(archive.Key), archive.Offset, archive.Size)
			if !suppressLog {
				log.Write("Downloading CASC file %d from archive %s", fileDataID, archive.Key)
			}
		} else {
			data, err = c.GetDataFile(c.FormatCDNKey(encodingKeyHex))
			if !suppressLog {
				log.Write("Downloading unarchived CASC file %d", fileDataID)
			}
			if data == nil {
				return nil, fmt.Errorf("no remote unarchived/archive indexed for encoding key: %s", encodingKeyHex)
			}
		}
		if err != nil {
			return nil, err
		}
		go c.CacheStore.StoreFile(encodingKeyHex, data, constants.Cache.DirData)
	} else if !suppressLog {
		log.Write("Loaded CASC file %d from cache", fileDataID)
	}
	return NewBLTEReader(data, encodingKeyHex, partialDecrypt)
}

// GetProductList returns available products on the remote CDN.
func (c *CASCRemote) GetProductList() []string {
	var products []string
	for _, entry := range c.BuildsList {
		if entry == nil {
			continue
		}
		for _, product := range constants.Products {
			if product.Product == entry["Product"] {
				products = append(products, fmt.Sprintf("%s %s", product.Title, entry["VersionsName"]))
				break
			}
		}
	}
	return products
}

// Preload preloads requirements for CDN fallback usage.
func (c *CASCRemote) Preload(buildIndex int, cache *BuildCache) error {
	c.CurrentBuild = c.BuildsList[buildIndex]
	log.Write("Preloading remote CASC build: %v", c.CurrentBuild)
	if cache != nil {
		c.CacheStore = cache
	} else {
		c.CacheStore = NewBuildCache(c.CurrentBuild["BuildConfig"])
		if err := c.CacheStore.Init(); err != nil {
			return err
		}
	}
	if err := c.LoadServerConfig(); err != nil {
		return err
	}
	if err := c.ResolveCDNHost(); err != nil {
		return err
	}
	if err := c.LoadConfigs(); err != nil {
		return err
	}
	return c.LoadArchives()
}

// Load loads the CASC interface with the given build.
func (c *CASCRemote) Load(buildIndex int) error {
	c.ResetForLoad()
	c.ProgressTracker = runtime.CreateProgress(12)
	if err := c.Preload(buildIndex, nil); err != nil {
		return err
	}
	if err := c.LoadEncoding(); err != nil {
		return err
	}
	if err := c.LoadRoot(); err != nil {
		return err
	}
	if err := c.PrepareListfile(); err != nil {
		return err
	}
	if err := c.LoadListfile(); err != nil {
		return err
	}
	if err := c.InitializeComponents(); err != nil {
		return err
	}
	c.Loaded = true
	return nil
}

// LoadEncoding downloads and parses the encoding file.
func (c *CASCRemote) LoadEncoding() error {
	encKeys := strings.Fields(c.BuildConfigData["encoding"])
	encKey := encKeys[1]
	log.TimeLog()
	if err := c.ProgressTracker.Step("Loading encoding table"); err != nil {
		return err
	}
	encRaw, _ := c.CacheStore.GetFile(constants.Cache.BuildEncoding, "")
	if encRaw == nil {
		log.Write("Encoding for build %s not cached, downloading.", c.CacheStore.Key)
		var err error
		encRaw, err = c.GetDataFile(c.FormatCDNKey(encKey))
		if err != nil {
			return err
		}
		go c.CacheStore.StoreFile(constants.Cache.BuildEncoding, encRaw, "")
	} else {
		log.Write("Encoding for build %s cached locally.", c.CacheStore.Key)
	}
	log.TimeEnd("Loaded encoding table (%s)", formats.Filesize(encRaw.ByteLength()))
	log.TimeLog()
	if err := c.ProgressTracker.Step("Parsing encoding table"); err != nil {
		return err
	}
	if err := c.ParseEncodingFile(encRaw, encKey); err != nil {
		return err
	}
	log.TimeEnd("Parsed encoding table (%d entries)", c.EncodingTable.len())
	return nil
}

// LoadRoot downloads and parses the root file.
func (c *CASCRemote) LoadRoot() error {
	rootKey, ok := c.EncodingTable.lookupEncodingKey(CascKeyFromHex(c.BuildConfigData["root"]))
	if !ok {
		return fmt.Errorf("no encoding entry found for root key")
	}
	log.TimeLog()
	if err := c.ProgressTracker.Step("Loading root table"); err != nil {
		return err
	}
	rootKeyHex := CascKeyToHex(rootKey)
	root, _ := c.CacheStore.GetFile(constants.Cache.BuildRoot, "")
	if root == nil {
		log.Write("Root file for build %s not cached, downloading.", c.CacheStore.Key)
		var err error
		root, err = c.GetDataFile(c.FormatCDNKey(rootKeyHex))
		if err != nil {
			return err
		}
		go c.CacheStore.StoreFile(constants.Cache.BuildRoot, root, "")
	}
	log.TimeEnd("Loaded root file (%s)", formats.Filesize(root.ByteLength()))
	log.TimeLog()
	if err := c.ProgressTracker.Step("Parsing root file"); err != nil {
		return err
	}
	rootEntryCount, err := c.ParseRootFile(root, rootKeyHex)
	if err != nil {
		return err
	}
	log.TimeEnd("Parsed root file (%d entries, %d types)", rootEntryCount, len(c.RootTypesList))
	return nil
}

// LoadArchives downloads and parses archive files.
func (c *CASCRemote) LoadArchives() error {
	archiveKeys := strings.Fields(c.CDNConfigData["archives"])
	archiveCount := len(archiveKeys)
	log.TimeLog()
	if c.ProgressTracker != nil {
		if err := c.ProgressTracker.Step("Loading archives"); err != nil {
			return err
		}
	}
	if err := formats.Queue(archiveKeys, func(key string) error {
		return c.ParseArchiveIndex(key)
	}, 50); err != nil {
		return err
	}
	archiveTotalSize := 0
	for _, e := range strings.Fields(c.CDNConfigData["archivesIndexSize"]) {
		n, _ := strconv.Atoi(e)
		archiveTotalSize += n
	}
	log.TimeEnd("Loaded %d archives (%d entries, %s)", archiveCount, len(c.Archives), formats.Filesize(archiveTotalSize))
	return nil
}

// LoadServerConfig downloads CDN configuration for the selected region.
func (c *CASCRemote) LoadServerConfig() error {
	if c.ProgressTracker != nil {
		if err := c.ProgressTracker.Step("Fetching CDN configuration"); err != nil {
			return err
		}
	}
	serverConfigs, err := c.GetConfig(c.CurrentBuild["Product"], constants.Patch.ServerConfig)
	if err != nil {
		return err
	}
	log.Write("%v", serverConfigs)
	for _, entry := range serverConfigs {
		if entry["Name"] == c.Region {
			c.ServerConfig = entry
			return nil
		}
	}
	return fmt.Errorf("CDN config does not contain entry for region %s", c.Region)
}

// ParseArchiveIndex loads and parses an archive index.
func (c *CASCRemote) ParseArchiveIndex(key string) error {
	fileName := key + ".index"
	data, _ := c.CacheStore.GetFile(fileName, constants.Cache.DirIndexes)
	if data == nil {
		cdnKey := c.FormatCDNKey(key) + ".index"
		var err error
		data, err = c.GetDataFile(cdnKey)
		if err != nil {
			return err
		}
		go c.CacheStore.StoreFile(fileName, data, constants.Cache.DirIndexes)
	}
	data.Seek(-12)
	count := int(data.ReadInt32LE().(int64))
	if count*24 > data.ByteLength() {
		return fmt.Errorf("unable to parse archive, unexpected size: %d", data.ByteLength())
	}
	data.Seek(0)
	entries := make(map[string]archiveEntry, count)
	for i := 0; i < count; i++ {
		hash := data.ReadHexString(16)
		if hash == emptyHash {
			hash = data.ReadHexString(16)
		}
		entries[hash] = archiveEntry{
			Key:    key,
			Size:   int(data.ReadInt32BE().(int64)),
			Offset: int(data.ReadInt32BE().(int64)),
		}
	}
	c.archivesMu.Lock()
	for hash, entry := range entries {
		c.Archives[hash] = entry
	}
	c.archivesMu.Unlock()
	return nil
}

// GetDataFile downloads a data file from the CDN.
func (c *CASCRemote) GetDataFile(file string) (*buffer.Buffer, error) {
	return c.downloadDataFile(file, -1, -1)
}

// GetDataFilePartial downloads a partial chunk of a data file.
func (c *CASCRemote) GetDataFilePartial(file string, ofs, length int) (*buffer.Buffer, error) {
	return c.downloadDataFile(file, ofs, length)
}

func (c *CASCRemote) downloadDataFile(file string, ofs, length int) (*buffer.Buffer, error) {
	var lastErr error
	for _, host := range c.healthyCDNHosts(time.Now()) {
		data, err := formats.DownloadFile([]string{host + "data/" + file}, "", ofs, length, false)
		if err == nil {
			return data, nil
		}
		lastErr = err
		c.deferCDNHost(host, time.Now())
		log.Write("CASC data host failed, trying fallback: %s (%s)", host, err)
	}
	return nil, lastErr
}

func (c *CASCRemote) healthyCDNHosts(now time.Time) []string {
	c.cdnHostsMu.Lock()
	defer c.cdnHostsMu.Unlock()

	hosts := c.cdnHosts
	if len(hosts) == 0 {
		hosts = []string{c.Host}
	}
	ready := make([]string, 0, len(hosts))
	deferred := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if retryAt := c.cdnRetryAt[host]; retryAt.After(now) {
			deferred = append(deferred, host)
		} else {
			ready = append(ready, host)
		}
	}
	if len(ready) == 0 {
		return deferred
	}
	return append(ready, deferred...)
}

func (c *CASCRemote) deferCDNHost(host string, now time.Time) {
	c.cdnHostsMu.Lock()
	defer c.cdnHostsMu.Unlock()
	c.cdnRetryAt[host] = now.Add(30 * time.Second)
}

// LoadConfigs downloads CDNConfig and BuildConfig.
func (c *CASCRemote) LoadConfigs() error {
	if c.ProgressTracker != nil {
		if err := c.ProgressTracker.Step("Fetching build configurations"); err != nil {
			return err
		}
	}
	cdnHosts, err := DefaultCDNResolver.GetRankedHosts(c.Region, c.ServerConfig)
	if err != nil {
		return err
	}
	c.CDNConfigData, err = c.GetCDNConfig(c.CurrentBuild["CDNConfig"], cdnHosts)
	if err != nil {
		return err
	}
	c.BuildConfigData, err = c.GetCDNConfig(c.CurrentBuild["BuildConfig"], cdnHosts)
	return err
}

// ResolveCDNHost resolves the fastest CDN host.
func (c *CASCRemote) ResolveCDNHost() error {
	if c.ProgressTracker != nil {
		if err := c.ProgressTracker.Step("Locating fastest CDN server"); err != nil {
			return err
		}
	}
	hosts, err := DefaultCDNResolver.GetRankedHosts(c.Region, c.ServerConfig)
	if err != nil {
		return err
	}
	c.cdnHostsMu.Lock()
	c.cdnHosts = hosts
	c.cdnRetryAt = map[string]time.Time{}
	c.Host = hosts[0]
	c.cdnHostsMu.Unlock()
	return nil
}

// FormatCDNKey formats a CDN key for CDN requests.
func (c *CASCRemote) FormatCDNKey(key string) string {
	return key[:2] + "/" + key[2:4] + "/" + key
}

// GetBuildName returns the current build ID.
func (c *CASCRemote) GetBuildName() string { return c.CurrentBuild["VersionsName"] }

// GetBuildKey returns the build configuration key.
func (c *CASCRemote) GetBuildKey() string { return c.CurrentBuild["BuildConfig"] }
