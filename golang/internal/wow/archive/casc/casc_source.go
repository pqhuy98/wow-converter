package casc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/runtime"
)

const (
	encMagic  = 0x4E45
	rootMagic = 0x4D465354
)

// RootType describes a CASC root type entry.
type RootType struct {
	ContentFlags uint32
	LocaleFlags  uint32
}

// RootEntry maps rootTypeIdx -> content key.
type RootEntry map[int]CascKey

// CASC is the interface for local and remote CASC sources.
type CASC interface {
	GetFile(fileDataID int, partialDecrypt bool, suppressLog bool, supportFallback bool, forceFallback bool, contentKey CascKey) (*BLTEReader, error)
	GetDataFile(key string) (*buffer.Buffer, error)
	FormatCDNKey(key string) string
	GetBuildName() string
	GetBuildKey() string
	GetProductList() []string
	Load(buildIndex int) error
	Cache() *BuildCache
	IsRemote() bool
	IsLoaded() bool
	Locale() int
	EncodingEntryCount() int
	RootTypes() []RootType
	RootEntries() map[int]RootEntry
	BuildConfig() CDNConfigEntries
	CDNConfig() CDNConfigEntries
	Builds() []VersionConfigEntry
	Build() VersionConfigEntry
}

// BaseCASC contains shared CASC state and logic.
type BaseCASC struct {
	EncodingTable    *EncodingTable
	RootTypesList    []RootType
	RootEntriesMap   map[int]RootEntry
	IsRemoteSource   bool
	Loaded           bool
	LocaleValue      int
	ProgressTracker  runtime.Progress
	BuildConfigData  CDNConfigEntries
	CDNConfigData    CDNConfigEntries
	BuildsList       []VersionConfigEntry
	CurrentBuild     VersionConfigEntry
	CacheStore       *BuildCache
}

// NewBaseCASC creates base CASC state.
func NewBaseCASC(isRemote bool) *BaseCASC {
	locale := server.GetConfig().CascLocale
	if locale <= 0 {
		log.Write("Invalid locale set in configuration, defaulting to enUS")
		locale = LocaleEnUS
	}
	return &BaseCASC{
		EncodingTable:    newEncodingTable(16, 16),
		RootEntriesMap:   map[int]RootEntry{},
		IsRemoteSource:   isRemote,
		LocaleValue:      locale,
		ProgressTracker:  runtime.CreateProgress(0),
	}
}

func (b *BaseCASC) IsRemote() bool            { return b.IsRemoteSource }
func (b *BaseCASC) IsLoaded() bool            { return b.Loaded }
func (b *BaseCASC) Locale() int               { return b.LocaleValue }
func (b *BaseCASC) EncodingEntryCount() int { return b.EncodingTable.len() }
func (b *BaseCASC) RootTypes() []RootType     { return b.RootTypesList }
func (b *BaseCASC) RootEntries() map[int]RootEntry { return b.RootEntriesMap }
func (b *BaseCASC) BuildConfig() CDNConfigEntries { return b.BuildConfigData }
func (b *BaseCASC) CDNConfig() CDNConfigEntries { return b.CDNConfigData }
func (b *BaseCASC) Builds() []VersionConfigEntry { return b.BuildsList }
func (b *BaseCASC) Build() VersionConfigEntry { return b.CurrentBuild }
func (b *BaseCASC) Cache() *BuildCache        { return b.CacheStore }

// ResetForLoad clears parsed index state before reloading.
func (b *BaseCASC) ResetForLoad() {
	b.EncodingTable.reset()
	b.RootTypesList = nil
	b.RootEntriesMap = map[int]RootEntry{}
	b.Loaded = false
}

// GetValidRootEntries returns fileDataIDs matching the current locale.
func (b *BaseCASC) GetValidRootEntries() []int {
	var entries []int
	for fileDataID, entry := range b.RootEntriesMap {
		include := false
		for rootTypeIdx := range entry {
			rootType := b.RootTypesList[rootTypeIdx]
			if (rootType.LocaleFlags&uint32(b.LocaleValue)) != 0 && (rootType.ContentFlags&uint32(ContentLowViolence)) == 0 {
				include = true
				break
			}
		}
		if include {
			entries = append(entries, fileDataID)
		}
	}
	return entries
}

// GetInstallManifest retrieves the install manifest for this CASC instance.
func (b *BaseCASC) GetInstallManifest(getter func(key string) (*buffer.Buffer, error), formatCDNKey func(string) string) (*InstallManifest, error) {
	installKeys := strings.Fields(b.BuildConfigData["install"])
	var installKey CascKey
	if len(installKeys) == 1 {
		encKey, ok := b.EncodingTable.lookupEncodingKey(CascKeyFromHex(installKeys[0]))
		if !ok {
			return nil, fmt.Errorf("no encoding entry found for install key")
		}
		installKey = encKey
	} else {
		installKey = CascKeyFromHex(installKeys[1])
	}
	installKeyHex := CascKeyToHex(installKey)
	var raw *buffer.Buffer
	var err error
	if b.IsRemoteSource {
		raw, err = getter(formatCDNKey(installKeyHex))
	} else {
		raw, err = getter(installKeyHex)
	}
	if err != nil {
		return nil, err
	}
	reader, err := NewBLTEReader(raw, installKeyHex, false)
	if err != nil {
		return nil, err
	}
	return NewInstallManifest(reader)
}

// FileExists checks if a file exists by fileDataID.
func (b *BaseCASC) FileExists(fileDataID int) bool {
	root, ok := b.RootEntriesMap[fileDataID]
	if !ok {
		return false
	}
	return b.selectRootContentKey(root) != ""
}

func (b *BaseCASC) selectRootContentKey(root RootEntry) CascKey {
	pick := func(requireLocale, skipLowViolence bool) CascKey {
		for rootTypeIdx, key := range root {
			rootType := b.RootTypesList[rootTypeIdx]
			if skipLowViolence && (rootType.ContentFlags&uint32(ContentLowViolence)) != 0 {
				continue
			}
			if (rootType.ContentFlags & uint32(ContentDoNotLoad)) != 0 {
				continue
			}
			if requireLocale && (rootType.LocaleFlags&uint32(b.LocaleValue)) == 0 {
				continue
			}
			return key
		}
		return ""
	}
	if key := pick(true, true); key != "" {
		return key
	}
	if key := pick(true, false); key != "" {
		return key
	}
	if key := pick(false, true); key != "" {
		return key
	}
	return pick(false, false)
}

// GetEncodingKey obtains the encoding key for a fileDataID.
func (b *BaseCASC) GetEncodingKey(fileDataID int) (CascKey, error) {
	root, ok := b.RootEntriesMap[fileDataID]
	if !ok {
		return "", fmt.Errorf("fileDataID does not exist in root: %d", fileDataID)
	}
	contentKey := b.selectRootContentKey(root)
	if contentKey == "" {
		return "", fmt.Errorf("no root entry found for locale: %d", b.LocaleValue)
	}
	return b.GetEncodingKeyForContentKey(contentKey)
}

// GetEncodingKeyForContentKey maps a content key to an encoding key.
func (b *BaseCASC) GetEncodingKeyForContentKey(contentKey CascKey) (CascKey, error) {
	key := AsCascKey(string(contentKey))
	encodingKey, ok := b.EncodingTable.lookupEncodingKey(key)
	if !ok {
		return "", fmt.Errorf("no encoding entry found: %s", CascKeyToHex(key))
	}
	return encodingKey, nil
}

// GetFileByName obtains a file by filename.
func (b *BaseCASC) GetFileByName(source CASC, fileName string, partialDecrypt, suppressLog, supportFallback, forceFallback bool) (*BLTEReader, error) {
	var fileDataID int
	var ok bool
	if strings.HasPrefix(fileName, "unknown/") && !strings.Contains(fileName, ".") {
		parts := strings.Split(fileName, "/")
		if len(parts) > 1 {
			fileDataID, _ = strconv.Atoi(parts[1])
			ok = fileDataID != 0
		}
	} else {
		fileDataID, ok = GetByFilename(fileName)
	}
	if !ok {
		return nil, fmt.Errorf("file not mapping in listfile: %s", fileName)
	}
	return source.GetFile(fileDataID, partialDecrypt, suppressLog, supportFallback, forceFallback, "")
}

// PrepareListfile ensures listfile preloading is complete.
func (b *BaseCASC) PrepareListfile() error {
	if err := b.ProgressTracker.Step("Preparing listfiles..."); err != nil {
		return err
	}
	_, err := PrepareListfile()
	return err
}

// LoadListfile applies the preloaded listfile for the selected build.
func (b *BaseCASC) LoadListfile() error {
	if err := b.ProgressTracker.Step("Loading listfiles"); err != nil {
		return err
	}
	ApplyPreload(b.RootEntriesMap)
	return nil
}

// InitializeComponents runs registered load functions.
func (b *BaseCASC) InitializeComponents() error {
	if err := b.ProgressTracker.Step("Initializing components"); err != nil {
		return err
	}
	return runtime.RunLoadFuncs()
}

// ParseRootFile parses entries from a root file.
func (b *BaseCASC) ParseRootFile(data *buffer.Buffer, hash string) (int, error) {
	root, err := NewBLTEReader(data, hash, false)
	if err != nil {
		return 0, err
	}
	magic := root.ReadUInt32LE().(int64)
	if uint32(magic) == rootMagic {
		headerSize := int(root.ReadUInt32LE().(int64))
		version := int(root.ReadUInt32LE().(int64))
		if headerSize != 0x18 {
			version = 0
		} else if version != 1 && version != 2 {
			return 0, fmt.Errorf("unknown root version: %d", version)
		}
		var totalFileCount, namedFileCount int
		if version == 0 {
			totalFileCount = headerSize
			namedFileCount = version
			headerSize = 12
		} else {
			totalFileCount = int(root.ReadUInt32LE().(int64))
			namedFileCount = int(root.ReadUInt32LE().(int64))
		}
		root.Seek(headerSize)
		allowNamelessFiles := totalFileCount != namedFileCount
		for root.RemainingBytes() > 0 {
			numRecords := int(root.ReadUInt32LE().(int64))
			var contentFlags, localeFlags uint32
			if version == 0 || version == 1 {
				contentFlags = uint32(root.ReadUInt32LE().(int64))
				localeFlags = uint32(root.ReadUInt32LE().(int64))
			} else if version == 2 {
				localeFlags = uint32(root.ReadUInt32LE().(int64))
				cflags1 := uint32(root.ReadUInt32LE().(int64))
				cflags2 := uint32(root.ReadUInt32LE().(int64))
				cflags3 := uint32(root.ReadUInt8().(int64))
				contentFlags = cflags1 | cflags2 | (cflags3 << 17)
			}
			fileDataIDs := make([]int, numRecords)
			fileDataID := 0
			for i := 0; i < numRecords; i++ {
				nextID := fileDataID + int(root.ReadInt32LE().(int64))
				fileDataIDs[i] = nextID
				fileDataID = nextID + 1
			}
			for i := 0; i < numRecords; i++ {
				fdid := fileDataIDs[i]
				entry := b.RootEntriesMap[fdid]
				if entry == nil {
					entry = RootEntry{}
					b.RootEntriesMap[fdid] = entry
				}
				entry[len(b.RootTypesList)] = CascKey(root.ReadBinaryKey(16))
			}
			if !(allowNamelessFiles && (contentFlags&uint32(ContentNoNameHash)) != 0) {
				root.Move(8 * numRecords)
			}
			b.RootTypesList = append(b.RootTypesList, RootType{ContentFlags: contentFlags, LocaleFlags: localeFlags})
		}
	} else {
		root.Seek(0)
		for root.RemainingBytes() > 0 {
			numRecords := int(root.ReadUInt32LE().(int64))
			contentFlags := uint32(root.ReadUInt32LE().(int64))
			localeFlags := uint32(root.ReadUInt32LE().(int64))
			fileDataIDs := make([]int, numRecords)
			fileDataID := 0
			for i := 0; i < numRecords; i++ {
				nextID := fileDataID + int(root.ReadInt32LE().(int64))
				fileDataIDs[i] = nextID
				fileDataID = nextID + 1
			}
			for i := 0; i < numRecords; i++ {
				key := CascKey(root.ReadBinaryKey(16))
				root.Move(8)
				fdid := fileDataIDs[i]
				entry := b.RootEntriesMap[fdid]
				if entry == nil {
					entry = RootEntry{}
					b.RootEntriesMap[fdid] = entry
				}
				entry[len(b.RootTypesList)] = key
			}
			b.RootTypesList = append(b.RootTypesList, RootType{ContentFlags: contentFlags, LocaleFlags: localeFlags})
		}
	}
	return len(b.RootEntriesMap), nil
}

// ParseEncodingFile parses entries from an encoding file.
func (b *BaseCASC) ParseEncodingFile(data *buffer.Buffer, hash string) error {
	encoding, err := NewBLTEReader(data, hash, false)
	if err != nil {
		return err
	}
	magic := encoding.ReadUInt16LE().(int64)
	if uint16(magic) != encMagic {
		return fmt.Errorf("invalid encoding magic: %d", magic)
	}
	encoding.Move(1)
	hashSizeCKey := int(encoding.ReadUInt8().(int64))
	hashSizeEKey := int(encoding.ReadUInt8().(int64))
	b.EncodingTable.init(hashSizeCKey, hashSizeEKey)
	cKeyPageSize := int(encoding.ReadInt16BE().(int64)) * 1024
	encoding.Move(2)
	cKeyPageCount := int(encoding.ReadInt32BE().(int64))
	encoding.Move(4 + 1)
	specBlockSize := int(encoding.ReadInt32BE().(int64))
	encoding.Move(specBlockSize + (cKeyPageCount * (hashSizeCKey + 16)))
	pagesStart := encoding.Offset()
	for i := 0; i < cKeyPageCount; i++ {
		pageStart := pagesStart + (cKeyPageSize * i)
		encoding.Seek(pageStart)
		for encoding.Offset() < (pageStart + pagesStart) {
			keysCount := int(encoding.ReadUInt8().(int64))
			if keysCount == 0 {
				break
			}
			size := int(encoding.ReadInt40BE().(int64))
			cKey := CascKey(encoding.ReadBinaryKey(hashSizeCKey))
			eKey := CascKey(encoding.ReadBinaryKey(hashSizeEKey))
			b.EncodingTable.set(cKey, eKey, size)
			encoding.Move(hashSizeEKey * (keysCount - 1))
		}
	}
	return nil
}

// Cleanup runs cleanup once a CASC instance is no longer needed.
func (b *BaseCASC) Cleanup() {}

// ReadInt40BE reads a 40-bit big-endian integer from BLTEReader.
func (r *BLTEReader) ReadInt40BE(count ...int) any {
	n := 5
	if len(count) > 0 {
		n = 5 * count[0]
	}
	r.CheckBounds(n)
	return r.Buffer.ReadInt40BE(count...)
}

// ReadUInt8Array is a helper alias for multi-byte reads used by install manifest.
func ReadUInt8Array(b *buffer.Buffer, count int) []byte {
	vals := b.ReadUInt8(count).([]int64)
	out := make([]byte, len(vals))
	for i, v := range vals {
		out[i] = byte(v)
	}
	return out
}
