// Package constants holds shared WoW reader constants.
package constants

import (
	"os"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// ProductInfo describes a Blizzard product entry.
type ProductInfo struct {
	Product string
	Title   string
	Tag     string
}

func dataPath() string {
	if p := os.Getenv("WOW_DATA_PATH"); p != "" {
		return p
	}
	return filepath.Join(workspace.FindRepoRoot(), ".cache", "wow")
}

// ExportPath is the default directory for exported artifacts.
var ExportPath = workspace.DefaultExportDir()

// DataPath is the root cache/data directory.
var DataPath = dataPath()

// Version stamped into export artifacts.
const Version = "0.2.1"

// UserAgent is used for HTTP requests.
const UserAgent = "wow-converter/" + Version

// Cache paths and filenames.
type cacheConstants struct {
	Dir              string
	Size             string
	IntegrityFile    string
	DirBuilds        string
	DirIndexes       string
	DirData          string
	DirDBD           string
	DirListfile      string
	BuildManifest    string
	BuildListfile    string
	BuildEncoding    string
	BuildRoot        string
	ListfileData     string
	TactKeys         string
}

// Cache holds cache-related paths.
var Cache = func() cacheConstants {
	root := filepath.Join(DataPath, "casc")
	return cacheConstants{
		Dir:           root,
		IntegrityFile: filepath.Join(root, "cacheintegrity"),
		DirBuilds:     filepath.Join(root, "builds"),
		DirIndexes:    filepath.Join(root, "indices"),
		DirData:       filepath.Join(root, "data"),
		DirDBD:        filepath.Join(root, "dbd"),
		DirListfile:   filepath.Join(root, "listfile"),
		BuildManifest: "manifest.json",
		BuildListfile: "listfile",
		BuildEncoding: "encoding",
		BuildRoot:     "root",
		ListfileData:  "listfile.txt",
		TactKeys:      filepath.Join(DataPath, "tact.json"),
	}
}()

// Products lists known Blizzard products.
var Products = []ProductInfo{
	{Product: "wow", Title: "World of Warcraft", Tag: "Retail"},
	{Product: "wowt", Title: "PTR: World of Warcraft", Tag: "PTR"},
	{Product: "wowxptr", Title: "PTR 2: World of Warcraft", Tag: "PTR 2"},
	{Product: "wow_beta", Title: "Beta: World of Warcraft", Tag: "Beta"},
	{Product: "wow_classic", Title: "World of Warcraft Classic", Tag: "Classic"},
	{Product: "wow_classic_beta", Title: "Beta: World of Warcraft Classic", Tag: "Classic Beta"},
	{Product: "wow_classic_ptr", Title: "PTR: World of Warcraft Classic", Tag: "Classic PTR"},
	{Product: "wow_classic_era", Title: "World of Warcraft Classic Era", Tag: "Classic Era"},
	{Product: "wow_classic_era_ptr", Title: "PTR: World of Warcraft Classic Era", Tag: "Classic Era PTR"},
}

// Patch holds CDN patch server constants.
type patchConstants struct {
	DefaultRegion  string
	Host           string
	ServerConfig   string
	VersionConfig  string
}

// Patch holds patch server configuration.
var Patch = patchConstants{
	DefaultRegion: "us",
	Host:          "https://%s.version.battle.net/",
	ServerConfig:  "/cdns",
	VersionConfig: "/versions",
}

// Build holds local installation constants.
type buildConstants struct {
	Manifest string
	DataDir  string
}

// Build holds build-related constants.
var Build = buildConstants{
	Manifest: ".build.info",
	DataDir:  "Data",
}
