package api

import (
	"os"
	"strconv"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

const (
	outputDirRel       = "exported-assets"
	outputDirBrowseRel = "exported-assets-browse"
	recentExportsRel   = "recent-exports.json"
)

// Config holds converter server settings mirroring src/server/config.ts.
type Config struct {
	IsSharedHosting  bool
	IsDev            bool
	ServerDeployTime string
	OutputDir        string
	OutputDirBrowse  string
	RecentExports    string
	Port             int
	ListenHost       string
}

// LoadConfig reads environment and ensures output directories exist.
func LoadConfig() Config {
	isShared := os.Getenv("IS_SHARED_HOSTING") == "true"
	isDev := config.IsDev()

	outputDir := workspace.ResolveRepoPath(outputDirRel)
	outputDirBrowse := workspace.ResolveRepoPath(outputDirBrowseRel)
	_ = os.MkdirAll(outputDir, 0o755)
	_ = os.MkdirAll(outputDirBrowse, 0o755)

	port := 3001
	if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			port = p
		}
	}

	return Config{
		IsSharedHosting:  isShared,
		IsDev:            isDev,
		ServerDeployTime: strconv.FormatInt(time.Now().UnixMilli(), 10),
		OutputDir:        outputDir,
		OutputDirBrowse:  outputDirBrowse,
		RecentExports:    workspace.ResolveRepoPath(recentExportsRel),
		Port:             port,
		ListenHost:       listenHost(isShared),
	}
}

func listenHost(isSharedHosting bool) string {
	if v := os.Getenv("HOST"); v != "" {
		return v
	}
	if isSharedHosting {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}
