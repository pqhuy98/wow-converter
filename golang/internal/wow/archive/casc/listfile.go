package casc

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/stringsort"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/formats"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

var (
	nameLookup        = map[string]int{}
	idLookup          = map[int]string{}
	listfileLoaded    bool
	preloadedIDLookup = map[int]string{}
	isPreloaded       bool
	preloadMu         sync.Mutex
)

type unknownIDProvider func() ([]int, error)

var (
	unknownModelProvider   unknownIDProvider
	unknownTextureProvider unknownIDProvider
)

func replaceExtension(file, ext string) string {
	return filepath.Join(filepath.Dir(file), strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))+ext)
}

func doPreload(rootEntries map[int]RootEntry) (bool, error) {
	log.Write("Preloading master listfile...")
	url := server.GetConfig().ListfileURL
	if url == "" {
		return false, errMissingListfileURL
	}
	if err := os.MkdirAll(constants.Cache.DirListfile, 0o755); err != nil {
		return false, err
	}
	cacheFile := filepath.Join(constants.Cache.DirListfile, constants.Cache.ListfileData)
	preloadedIDLookup = make(map[int]string, len(rootEntries))
	isPreloaded = false

	file := url
	if strings.HasPrefix(url, "http") {
		file = cacheFile
		cached, statErr := os.Stat(cacheFile)
		ttl := time.Duration(server.GetConfig().ListfileCacheRefresh) * 24 * time.Hour
		requireDownload := statErr != nil || ttl == 0 || time.Since(cached.ModTime()) > ttl
		if requireDownload {
			fallbackURL := strings.ReplaceAll(server.GetConfig().ListfileFallbackURL, "%s", "")
			err := formats.DownloadFileToDisk([]string{url, fallbackURL}, cacheFile)
			if err != nil {
				if statErr != nil {
					log.Write("Failed to download listfile during preload, no cached version for fallback: %s", err.Error())
					return false, nil
				}
				log.Write("Failed to download listfile during preload, using cached version: %s", err.Error())
			}
		} else {
			log.Write("Listfile is cached locally.")
		}
	} else {
		log.Write("Preloading user-defined local listfile: %s", url)
	}
	reader, err := os.Open(file)
	if err != nil {
		return false, err
	}
	defer reader.Close()
	if err := parseMasterListfileReader(reader, rootEntries); err != nil {
		return false, err
	}
	if len(preloadedIDLookup) == 0 {
		log.Write("No entries found in preloaded listfile")
		return false, nil
	}
	isPreloaded = true
	log.Write("Preloaded %d listfile entries", len(preloadedIDLookup))
	return true, nil
}

func splitListfileLines(data []byte, atEOF bool) (int, []byte, error) {
	if end := bytes.IndexAny(data, "\r\n"); end >= 0 {
		return end + 1, data[:end], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func parseMasterListfileReader(reader io.Reader, rootEntries map[int]RootEntry) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	scanner.Split(splitListfileLines)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		id, name, found := bytes.Cut(line, []byte(";"))
		if !found || bytes.ContainsRune(name, ';') {
			log.Write("Invalid listfile line (token count): %s", line)
			continue
		}
		fileDataID, err := strconv.Atoi(string(id))
		if err != nil {
			log.Write("Invalid listfile line (non-numerical ID): %s", line)
			continue
		}
		if rootEntries != nil {
			if _, ok := rootEntries[fileDataID]; !ok {
				continue
			}
		}
		fileName := strings.ToLower(string(name))
		preloadedIDLookup[fileDataID] = fileName
	}
	return scanner.Err()
}

// PrepareListfile reads only names belonging to the selected build. Loading it
// after the root avoids holding a master map throughout encoding/root decoding.
func PrepareListfile(rootEntries map[int]RootEntry) (bool, error) {
	preloadMu.Lock()
	defer preloadMu.Unlock()
	if isPreloaded {
		return true, nil
	}
	return doPreload(rootEntries)
}

// ApplyPreload applies preloaded listfile data filtered by rootEntries.
func ApplyPreload(rootEntries map[int]RootEntry) int {
	if !isPreloaded {
		log.Write("No preloaded listfile available, falling back to normal loading")
		return 0
	}
	log.Write("Applying preloaded listfile data...")
	// Transfer ownership rather than copy millions of entries into another map.
	for fileDataID := range preloadedIDLookup {
		if _, ok := rootEntries[fileDataID]; !ok {
			delete(preloadedIDLookup, fileDataID)
		}
	}
	appliedCount := len(preloadedIDLookup)
	if appliedCount == 0 {
		log.Write("No preloaded entries matched rootEntries")
		return 0
	}
	idLookup = preloadedIDLookup
	nameLookup = make(map[string]int, appliedCount)
	for fileDataID, fileName := range idLookup {
		nameLookup[fileName] = fileDataID
	}
	listfileLoaded = true
	log.Write("Applied %d preloaded listfile entries", appliedCount)
	preloadedIDLookup = map[int]string{}
	isPreloaded = false
	return appliedCount
}

// SetUnknownModelProvider registers a provider of unknown model fileDataIDs.
func SetUnknownModelProvider(provider unknownIDProvider) {
	unknownModelProvider = provider
}

// SetUnknownTextureProvider registers a provider of unknown texture fileDataIDs.
func SetUnknownTextureProvider(provider unknownIDProvider) {
	unknownTextureProvider = provider
}

// LoadUnknownTextures loads unknown texture files.
func LoadUnknownTextures() (int, error) {
	if unknownTextureProvider == nil {
		return 0, nil
	}
	ids, err := unknownTextureProvider()
	if err != nil {
		return 0, err
	}
	count := LoadIDTable(ids, ".blp")
	log.Write("Added %d unknown BLP textures from TextureFileData to listfile", count)
	return count, nil
}

// LoadUnknownModels loads unknown model files.
func LoadUnknownModels() (int, error) {
	if unknownModelProvider == nil {
		return 0, nil
	}
	ids, err := unknownModelProvider()
	if err != nil {
		return 0, err
	}
	count := LoadIDTable(ids, ".m2")
	log.Write("Added %d unknown M2 models from ModelFileData to listfile", count)
	return count, nil
}

// LoadUnknowns loads unknown files from ModelFileData.
func LoadUnknowns() error {
	_, err := LoadUnknownModels()
	return err
}

// LoadIDTable loads file IDs from a data table.
func LoadIDTable(ids []int, ext string) int {
	loadCount := 0
	for _, fileDataID := range ids {
		if _, ok := idLookup[fileDataID]; !ok {
			fileName := FormatUnknownFile(fileDataID, ext)
			idLookup[fileDataID] = fileName
			nameLookup[fileName] = fileDataID
			loadCount++
		}
	}
	return loadCount
}

// ExtensionFilter filters listfile entries by extension.
type ExtensionFilter any

// GetFilenamesByExtension returns filenames ending with the given extension(s).
func GetFilenamesByExtension(exts ExtensionFilter) []string {
	filters := []ExtensionFilter{exts}
	if arr, ok := exts.([]ExtensionFilter); ok {
		filters = arr
	}
	var entries []int
	for fileDataID, filename := range idLookup {
		for _, ext := range filters {
			switch e := ext.(type) {
			case [2]any:
				suffix, _ := e[0].(string)
				re, _ := e[1].(*regexp.Regexp)
				if strings.HasSuffix(filename, suffix) && (re == nil || !re.MatchString(filename)) {
					entries = append(entries, fileDataID)
				}
			case string:
				if strings.HasSuffix(filename, e) {
					entries = append(entries, fileDataID)
				}
			}
		}
	}
	return FormatEntries(entries)
}

// FormatEntries sorts and formats listfile entries.
func FormatEntries(entries []int) []string {
	cfg := server.GetConfig()
	if cfg.ListfileSortByID {
		sort.Ints(entries)
	}
	formatted := make([]string, len(entries))
	for i, e := range entries {
		if cfg.ListfileShowFileDataIDs {
			formatted[i] = GetByIDOrUnknown(e, "") + " [" + strconv.Itoa(e) + "]"
		} else {
			formatted[i] = GetByIDOrUnknown(e, "")
		}
	}
	if !cfg.ListfileSortByID {
		sort.Strings(formatted)
	}
	return formatted
}

// IngestIdentifiedFiles adds identified unknown files.
func IngestIdentifiedFiles(entries map[int]string) {
	for fileDataID, ext := range entries {
		fileName := FormatUnknownFile(fileDataID, ext)
		idLookup[fileDataID] = fileName
		nameLookup[fileName] = fileDataID
	}
}

// GetFullListfile returns a full listfile.
func GetFullListfile() []string {
	keys := make([]int, 0, len(idLookup))
	for k := range idLookup {
		keys = append(keys, k)
	}
	return FormatEntries(keys)
}

// GetByID returns a filename for a file data ID.
func GetByID(id int) (string, bool) {
	name, ok := idLookup[id]
	return name, ok
}

// GetByIDOrUnknown returns a filename or unknown path.
func GetByIDOrUnknown(id int, ext string) string {
	if name, ok := idLookup[id]; ok {
		return name
	}
	return FormatUnknownFile(id, ext)
}

// GetByFilename returns a file data ID by filename.
func GetByFilename(filename string) (int, bool) {
	lookup, ok := nameLookup[strings.ToLower(strings.ReplaceAll(filename, "\\", "/"))]
	if !ok && (strings.HasSuffix(filename, ".mdl") || strings.HasSuffix(filename, "mdx")) {
		alt := strings.ReplaceAll(replaceExtension(filename, ".m2"), "\\", "/")
		lookup, ok = nameLookup[strings.ToLower(alt)]
	}
	return lookup, ok
}

// ListfileEntry is a filtered listfile entry.
type ListfileEntry struct {
	FileDataID int
	FileName   string
}

// GetFilteredEntries returns listfile entries matching search.
func GetFilteredEntries(search any) []ListfileEntry {
	var results []ListfileEntry
	isRegExp := false
	var re *regexp.Regexp
	var term string
	switch s := search.(type) {
	case *regexp.Regexp:
		isRegExp = true
		re = s
	case string:
		term = s
	}
	for fileDataID, fileName := range idLookup {
		match := false
		if isRegExp {
			match = re.MatchString(fileName)
		} else {
			match = term == "" || strings.Contains(fileName, term)
		}
		if match {
			results = append(results, ListfileEntry{FileDataID: fileDataID, FileName: fileName})
		}
	}
	return results
}

// StripFileEntry strips a prefixed file ID from a listfile entry.
func StripFileEntry(entry string) string {
	if strings.Contains(entry, " [") {
		return entry[:strings.LastIndex(entry, " [")]
	}
	return entry
}

// FormatUnknownFile returns a file path for an unknown fileDataID.
func FormatUnknownFile(fileDataID int, ext string) string {
	return "unknown/" + strconv.Itoa(fileDataID) + ext
}

// IsListfileLoaded reports whether a listfile has been loaded.
func IsListfileLoaded() bool {
	return listfileLoaded
}

// ListfileMemoryStats reports in-memory listfile sizes.
type ListfileMemoryStats struct {
	IDLookup            int
	NameLookup          int
	PreloadedIDLookup   int
	PreloadedNameLookup int
	IsPreloaded         bool
	Loaded              bool
}

// GetMemoryStats returns listfile memory stats.
func GetMemoryStats() ListfileMemoryStats {
	return ListfileMemoryStats{
		IDLookup:            len(idLookup),
		NameLookup:          len(nameLookup),
		PreloadedIDLookup:   len(preloadedIDLookup),
		PreloadedNameLookup: 0, // No reverse map is needed before build selection.
		IsPreloaded:         isPreloaded,
		Loaded:              listfileLoaded,
	}
}

// IndexMemoryStats reports built browse/map tile indexes held in RAM.
type IndexMemoryStats struct {
	BrowseBuilt    bool
	BrowseModels   int
	BrowseTextures int
	MapTileBuilt   bool
	MapTileEntries int
}

// GetIndexMemoryStats returns browse and map-tile index sizes.
func GetIndexMemoryStats() IndexMemoryStats {
	browseIndexMu.Lock()
	bm, bt := browseModels, browseTextures
	browseIndexMu.Unlock()

	mapTileIndexMu.Lock()
	mt := mapTileFileIndex
	mapTileIndexMu.Unlock()

	stats := IndexMemoryStats{
		BrowseBuilt:  bm != nil,
		MapTileBuilt: mt != nil,
	}
	if bm != nil {
		stats.BrowseModels = len(bm)
	}
	if bt != nil {
		stats.BrowseTextures = len(bt)
	}
	if mt != nil {
		stats.MapTileEntries = len(mt)
	}
	return stats
}

// AddEntry adds an entry to the listfile.
func AddEntry(fileDataID int, fileName string) {
	idLookup[fileDataID] = fileName
	nameLookup[fileName] = fileDataID
}

// ResetForCascUnload clears in-memory listfile state.
func ResetForCascUnload() {
	listfileLoaded = false
	isPreloaded = false
	idLookup = map[int]string{}
	nameLookup = map[string]int{}
	preloadedIDLookup = map[int]string{}
	resetFileIndexes()
}

var (
	browseM2WmoRegex   = regexp.MustCompile(`(?i)\.(m2|wmo)$`)
	browseBadWmoRegex  = regexp.MustCompile(`(?i)_([0-9]{3}|lod\d)\.wmo$`)
	browseTextureRegex = regexp.MustCompile(`(?i)\.(blp|png|tga|dds)$`)
	mapTileBlpRegex    = regexp.MustCompile(`^world/minimaps/([^/]+)/map(\d{1,2})_(\d{1,2})\.blp$`)
	mapTileAdtRegex    = regexp.MustCompile(`(?i)^world/maps/([^/]+)/([^/]+)_(\d{2})_(\d{2})\.adt$`)
)

var (
	browseIndexMu    sync.Mutex
	browseModels     []ListfileEntry
	browseTextures   []ListfileEntry
	mapTileIndexMu   sync.Mutex
	mapTileFileIndex []ListfileEntry
)

func resetFileIndexes() {
	browseIndexMu.Lock()
	browseModels = nil
	browseTextures = nil
	browseIndexMu.Unlock()

	mapTileIndexMu.Lock()
	mapTileFileIndex = nil
	mapTileIndexMu.Unlock()
}

func sortBrowseFileIndex(models, textures []ListfileEntry) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		stringsort.SortBy(models, func(e ListfileEntry) string { return e.FileName })
		wg.Done()
	}()
	go func() {
		stringsort.SortBy(textures, func(e ListfileEntry) string { return e.FileName })
		wg.Done()
	}()
	wg.Wait()
}

func buildBrowseFileIndex() (models, textures []ListfileEntry) {
	models = make([]ListfileEntry, 0, 150000)
	textures = make([]ListfileEntry, 0, 800000)
	for fileDataID, fileName := range idLookup {
		if browseBadWmoRegex.MatchString(fileName) {
			continue
		}
		if browseM2WmoRegex.MatchString(fileName) {
			models = append(models, ListfileEntry{FileDataID: fileDataID, FileName: fileName})
		} else if browseTextureRegex.MatchString(fileName) {
			textures = append(textures, ListfileEntry{FileDataID: fileDataID, FileName: fileName})
		}
	}
	sortBrowseFileIndex(models, textures)
	return models, textures
}

// CollectBrowseFileIndex scans the loaded listfile once for browse model/texture entries.
func CollectBrowseFileIndex() (models, textures []ListfileEntry) {
	browseIndexMu.Lock()
	defer browseIndexMu.Unlock()
	if browseModels != nil {
		return browseModels, browseTextures
	}
	browseModels, browseTextures = buildBrowseFileIndex()
	return browseModels, browseTextures
}

func buildMapTileFileIndex() []ListfileEntry {
	entries := make([]ListfileEntry, 0, 50000)
	for fileDataID, fileName := range idLookup {
		lower := strings.ToLower(strings.ReplaceAll(fileName, `\`, `/`))
		if mapTileBlpRegex.MatchString(lower) || mapTileAdtRegex.MatchString(lower) {
			entries = append(entries, ListfileEntry{FileDataID: fileDataID, FileName: fileName})
		}
	}
	return entries
}

// CollectMapTileFileIndex scans the loaded listfile for minimap and ADT tile entries.
func CollectMapTileFileIndex() []ListfileEntry {
	mapTileIndexMu.Lock()
	defer mapTileIndexMu.Unlock()
	if mapTileFileIndex != nil {
		return mapTileFileIndex
	}
	mapTileFileIndex = buildMapTileFileIndex()
	return mapTileFileIndex
}
