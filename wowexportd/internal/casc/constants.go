package casc

import (
	"os"
	"path/filepath"
)

// Patch server constants mirroring wow.export
const (
	PatchHostFormat    = "https://%s.version.battle.net/"
	PatchServerConfig  = "/cdns"
	PatchVersionConfig = "/versions"
)

// Cache directory layout under ~/.wowexportd/casc
func userDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wowexportd")
}

func cacheRoot() string { return filepath.Join(userDataDir(), "casc") }

func DirBuilds() string    { return filepath.Join(cacheRoot(), "builds") }
func DirIndexes() string   { return filepath.Join(cacheRoot(), "indices") }
func DirData() string      { return filepath.Join(cacheRoot(), "data") }
func DirListfile() string  { return filepath.Join(cacheRoot(), "listfile") }
func PathTACTKeys() string { return filepath.Join(cacheRoot(), "tact.json") }
