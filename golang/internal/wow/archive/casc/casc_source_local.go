package casc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/formats"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/runtime"
)

type localIndexEntry struct {
	index  int
	offset int
	size   int
}

// CASCLocal is a local installation CASC source.
type CASCLocal struct {
	*BaseCASC
	Dir          string
	DataDir      string
	StorageDir   string
	LocalIndexes map[string]localIndexEntry
	Remote       *CASCRemote
}

// NewCASCLocal creates a local CASC source.
func NewCASCLocal(dir string) *CASCLocal {
	base := NewBaseCASC(false)
	dataDir := filepath.Join(dir, constants.Build.DataDir)
	return &CASCLocal{
		BaseCASC:     base,
		Dir:          dir,
		DataDir:      dataDir,
		StorageDir:   filepath.Join(dataDir, "data"),
		LocalIndexes: map[string]localIndexEntry{},
	}
}

// Init initializes the local CASC source.
func (c *CASCLocal) Init() error {
	log.Write("Initializing local CASC installation: %s", c.Dir)
	buildInfo := filepath.Join(c.Dir, constants.Build.Manifest)
	data, err := os.ReadFile(buildInfo)
	if err != nil {
		return err
	}
	config := ParseVersionConfig(string(data))
	for _, entry := range config {
		for _, product := range constants.Products {
			if product.Product == entry["Product"] {
				c.BuildsList = append(c.BuildsList, entry)
				break
			}
		}
	}
	return nil
}

// GetFile obtains a file by fileDataID.
func (c *CASCLocal) GetFile(fileDataID int, partialDecrypt, suppressLog, supportFallback, forceFallback bool, contentKey CascKey) (*BLTEReader, error) {
	if !suppressLog {
		name, _ := GetByID(fileDataID)
		log.Write("Loading local CASC file %d (%s)", fileDataID, name)
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
	var data *buffer.Buffer
	if supportFallback {
		data, err = c.GetDataFileWithRemoteFallback(encodingKeyHex, forceFallback)
	} else {
		data, err = c.GetDataFile(encodingKeyHex)
	}
	if err != nil {
		return nil, err
	}
	return NewBLTEReader(data, encodingKeyHex, partialDecrypt)
}

// GetProductList returns available products.
func (c *CASCLocal) GetProductList() []string {
	var products []string
	for _, entry := range c.BuildsList {
		for _, product := range constants.Products {
			if product.Product == entry["Product"] {
				products = append(products, fmt.Sprintf("%s (%s) %s", product.Title, strings.ToUpper(entry["Branch"]), entry["Version"]))
				break
			}
		}
	}
	return products
}

// Load loads the CASC interface with the given build.
func (c *CASCLocal) Load(buildIndex int) error {
	c.ResetForLoad()
	c.LocalIndexes = map[string]localIndexEntry{}
	c.CurrentBuild = c.BuildsList[buildIndex]
	log.Write("Loading local CASC build: %v", c.CurrentBuild)
	c.CacheStore = NewBuildCache(c.CurrentBuild["BuildKey"])
	if err := c.CacheStore.Init(); err != nil {
		return err
	}
	c.ProgressTracker = runtime.CreateProgress(8)
	if err := c.LoadConfigs(); err != nil {
		return err
	}
	if err := c.LoadIndexes(); err != nil {
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

// LoadConfigs loads build and CDN configs.
func (c *CASCLocal) LoadConfigs() error {
	if err := c.ProgressTracker.Step("Fetching build configurations"); err != nil {
		return err
	}
	if formats.FileExists("fakebuildconfig") {
		data, err := os.ReadFile("fakebuildconfig")
		if err != nil {
			return err
		}
		cfg, err := ParseCDNConfig(string(data))
		if err != nil {
			return err
		}
		c.BuildConfigData = cfg
		log.Write("WARNING: Using fake build config. No support given for weird stuff happening.")
		splitName := strings.Split(c.BuildConfigData["buildName"], "patch")
		buildNumber := strings.TrimPrefix(splitName[0], "WOW-")
		splitPatch := strings.Split(splitName[1], "_")
		c.CurrentBuild["Version"] = splitPatch[0] + "." + buildNumber
	} else {
		cfg, err := c.GetConfigFileWithRemoteFallback(c.CurrentBuild["BuildKey"])
		if err != nil {
			return err
		}
		c.BuildConfigData = cfg
	}
	cfg, err := c.GetConfigFileWithRemoteFallback(c.CurrentBuild["CDNKey"])
	if err != nil {
		return err
	}
	c.CDNConfigData = cfg
	return nil
}

// GetConfigFileWithRemoteFallback gets config from disk with CDN fallback.
func (c *CASCLocal) GetConfigFileWithRemoteFallback(key string) (CDNConfigEntries, error) {
	configPath := c.FormatConfigPath(key)
	if !formats.FileExists(configPath) {
		log.Write("Local config file %s does not exist, falling back to CDN...", key)
		if c.Remote == nil {
			if err := c.InitializeRemoteCASC(); err != nil {
				return nil, err
			}
		}
		cdnHosts, err := DefaultCDNResolver.GetRankedHosts(runtime.RuntimeState.SelectedCDNRegionTag, c.Remote.ServerConfig)
		if err != nil {
			return nil, err
		}
		return c.Remote.GetCDNConfig(key, cdnHosts)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	return ParseCDNConfig(string(data))
}

// LoadIndexes loads and parses storage indexes.
func (c *CASCLocal) LoadIndexes() error {
	log.TimeLog()
	if err := c.ProgressTracker.Step("Loading indexes"); err != nil {
		return err
	}
	indexCount := 0
	entries, err := os.ReadDir(c.StorageDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".idx") {
			if err := c.ParseIndex(filepath.Join(c.StorageDir, entry.Name())); err != nil {
				return err
			}
			indexCount++
		}
	}
	log.TimeEnd("Loaded %d entries from %d journal indexes", len(c.LocalIndexes), indexCount)
	return nil
}

// ParseIndex parses a local journal index.
func (c *CASCLocal) ParseIndex(file string) error {
	index, err := buffer.ReadFile(file)
	if err != nil {
		return err
	}
	headerHashSize := int(index.ReadInt32LE().(int64))
	index.Move(4)
	index.Move(headerHashSize)
	index.Seek((8 + headerHashSize + 0x0F) & 0xFFFFFFF0)
	dataLength := int(index.ReadInt32LE().(int64))
	index.Move(4)
	nBlocks := dataLength / 18
	for i := 0; i < nBlocks; i++ {
		key := index.ReadHexString(9)
		if _, exists := c.LocalIndexes[key]; exists {
			index.Move(1 + 4 + 4)
			continue
		}
		idxHigh := int(index.ReadUInt8().(int64))
		idxLow := int(index.ReadInt32BE().(int64))
		c.LocalIndexes[key] = localIndexEntry{
			index:  (idxHigh << 2) | ((idxLow & 0xC0000000) >> 30),
			offset: idxLow & 0x3FFFFFFF,
			size:   int(index.ReadInt32LE().(int64)),
		}
	}
	return nil
}

// LoadEncoding loads and parses the encoding table.
func (c *CASCLocal) LoadEncoding() error {
	log.TimeLog()
	encKeys := strings.Fields(c.BuildConfigData["encoding"])
	if err := c.ProgressTracker.Step("Loading encoding table"); err != nil {
		return err
	}
	encRaw, err := c.GetDataFileWithRemoteFallback(encKeys[1], false)
	if err != nil {
		return err
	}
	if err := c.ParseEncodingFile(encRaw, encKeys[1]); err != nil {
		return err
	}
	log.TimeEnd("Parsed encoding table (%d entries)", c.EncodingTable.len())
	return nil
}

// LoadRoot loads and parses the root table.
func (c *CASCLocal) LoadRoot() error {
	rootKey, ok := c.EncodingTable.lookupEncodingKey(CascKeyFromHex(c.BuildConfigData["root"]))
	if !ok {
		return fmt.Errorf("no encoding entry found for root key")
	}
	log.TimeLog()
	if err := c.ProgressTracker.Step("Loading root file"); err != nil {
		return err
	}
	rootKeyHex := CascKeyToHex(rootKey)
	root, err := c.GetDataFileWithRemoteFallback(rootKeyHex, false)
	if err != nil {
		return err
	}
	rootEntryCount, err := c.ParseRootFile(root, rootKeyHex)
	if err != nil {
		return err
	}
	log.TimeEnd("Parsed root file (%d entries, %d types)", rootEntryCount, len(c.RootTypesList))
	return nil
}

// InitializeRemoteCASC initializes a remote CASC for CDN fallback.
func (c *CASCLocal) InitializeRemoteCASC() error {
	remote := NewCASCRemote(runtime.RuntimeState.SelectedCDNRegionTag)
	if err := remote.Init(); err != nil {
		return err
	}
	buildIndex := -1
	for i, build := range remote.BuildsList {
		if build["Product"] == c.CurrentBuild["Product"] {
			buildIndex = i
			break
		}
	}
	if buildIndex < 0 {
		return fmt.Errorf("remote build not found for product %s", c.CurrentBuild["Product"])
	}
	if err := remote.Preload(buildIndex, c.CacheStore); err != nil {
		return err
	}
	c.Remote = remote
	return nil
}

// GetDataFileWithRemoteFallback obtains a data file with CDN fallback.
func (c *CASCLocal) GetDataFileWithRemoteFallback(key string, forceFallback bool) (*buffer.Buffer, error) {
	tryLocal := func() (*buffer.Buffer, error) {
		if forceFallback {
			return nil, fmt.Errorf("local data is corrupted, forceFallback set")
		}
		local, err := c.GetDataFile(key)
		if err != nil {
			return nil, err
		}
		if !CheckBLTE(local) {
			return nil, fmt.Errorf("local data file is not a valid BLTE")
		}
		return local, nil
	}
	local, err := tryLocal()
	if err == nil {
		return local, nil
	}
	log.Write("Local data file %s does not exist, falling back to cache...", key)
	cached, _ := c.CacheStore.GetFile(key, constants.Cache.DirData)
	if cached != nil {
		return cached, nil
	}
	log.Write("Local data file %s not cached, falling back to CDN...", key)
	if c.Remote == nil {
		if err := c.InitializeRemoteCASC(); err != nil {
			return nil, err
		}
	}
	remote := c.Remote
	archive, ok := remote.Archives[key]
	var data *buffer.Buffer
	if ok {
		log.Write("Local data file %s has archive, attempt partial download...", key)
		data, err = remote.GetDataFilePartial(remote.FormatCDNKey(archive.Key), archive.Offset, archive.Size)
	} else {
		log.Write("Local data file %s has no archive, attempting direct download...", key)
		data, err = remote.GetDataFile(remote.FormatCDNKey(key))
	}
	if err != nil {
		return nil, err
	}
	go c.CacheStore.StoreFile(key, data, constants.Cache.DirData)
	return data, nil
}

// GetDataFile obtains a data file from local archives.
func (c *CASCLocal) GetDataFile(key string) (*buffer.Buffer, error) {
	entry, ok := c.LocalIndexes[key[:18]]
	if !ok {
		return nil, fmt.Errorf("requested file does not exist in local data: %s", key)
	}
	data, err := formats.ReadFile(c.FormatDataPath(entry.index), entry.offset+0x1E, entry.size-0x1E)
	if err != nil {
		return nil, err
	}
	isZeroed := true
	data.Seek(0)
	for i := 0; i < data.RemainingBytes(); i++ {
		if data.ReadUInt8().(int64) != 0 {
			isZeroed = false
			break
		}
	}
	if isZeroed {
		return nil, fmt.Errorf("requested data file is empty or missing: %s", key)
	}
	data.Seek(0)
	return data, nil
}

// FormatDataPath formats a local path to a data archive.
func (c *CASCLocal) FormatDataPath(id int) string {
	return filepath.Join(c.DataDir, "data", fmt.Sprintf("data.%03d", id))
}

// FormatIndexPath formats a local path to an archive index.
func (c *CASCLocal) FormatIndexPath(key string) string {
	return filepath.Join(c.DataDir, "indices", key+".index")
}

// FormatConfigPath formats a local path to a config file.
func (c *CASCLocal) FormatConfigPath(key string) string {
	return filepath.Join(c.DataDir, "config", c.FormatCDNKey(key))
}

// FormatCDNKey formats a CDN key for local file reading.
func (c *CASCLocal) FormatCDNKey(key string) string {
	return filepath.Join(key[:2], key[2:4], key)
}

// GetBuildName returns the current build ID.
func (c *CASCLocal) GetBuildName() string { return c.CurrentBuild["Version"] }

// GetBuildKey returns the build configuration key.
func (c *CASCLocal) GetBuildKey() string { return c.CurrentBuild["BuildKey"] }
