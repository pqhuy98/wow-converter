package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"runtime/debug"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/server/pathsafe"
)

var buildSHA string // Set by release builds, including when .git is absent at runtime.

type exportReportMetadata struct {
	Request     exportCharacterRequest           `json:"request"`
	GitSHA      string                           `json:"gitSha"`
	BuildKey    string                           `json:"buildKey"`
	Product     string                           `json:"product"`
	ExportedAt  int64                            `json:"exportedAt"`
	Models      []character.ModelSequenceSources `json:"models"`
	AssetHashes map[string]string                `json:"assetHashes"`
}

type bugReportPayload struct {
	ID         string                   `json:"id"`
	WowheadURL string                   `json:"wowheadUrl"`
	ModelName  string                   `json:"modelName"`
	Request    exportCharacterRequest   `json:"request"`
	Sequence   character.SequenceSource `json:"sequence"`
	GitSHA     string                   `json:"gitSha"`
	BuildKey   string                   `json:"buildKey"`
	Product    string                   `json:"product"`
	ExportedAt int64                    `json:"exportedAt"`
	Notes      string                   `json:"notes"`
}

func currentGitSHA() string {
	if buildSHA != "" {
		return buildSHA
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return "unknown"
}

func hashExportAsset(base, path string) (string, error) {
	full, err := pathsafe.ResolveUnderBase(base, path)
	if err != nil {
		return "", err
	}
	file, err := os.Open(full)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func checkExportAssets(metadata *exportReportMetadata, base string) error {
	if len(metadata.AssetHashes) == 0 {
		return errors.New("Export again to prepare a report for this model.")
	}
	for path, want := range metadata.AssetHashes {
		got, err := hashExportAsset(base, path)
		if err != nil || got != want {
			return errors.New("Exported files were changed or cleaned. Export the model again before preparing screenshots.")
		}
	}
	return nil
}

func validWowheadURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "wowhead.com" || strings.HasSuffix(host, ".wowhead.com")
}

func validateReportPayload(payload bugReportPayload) error {
	if !validWowheadURL(payload.WowheadURL) || payload.Request.Character.Base.Type != "wowhead" || payload.Request.Character.Base.Value != payload.WowheadURL {
		return errors.New("A valid Wowhead source URL is required.")
	}
	if len(strings.TrimSpace(payload.Notes)) < 1 || len(payload.Notes) > 8000 {
		return errors.New("Describe what looks wrong (up to 8,000 characters).")
	}
	if payload.Sequence.WowName == "" || payload.Sequence.Name == "" || len(payload.Sequence.Name) > 160 || len(payload.Sequence.WowName) > 160 || payload.Sequence.WowVariant < 0 || payload.Sequence.WowVariant > 65535 {
		return errors.New("Select an animation with a matching Wowhead sequence.")
	}
	if len(payload.GitSHA) > 80 || len(payload.BuildKey) > 128 || len(payload.Product) > 40 {
		return errors.New("Invalid export metadata.")
	}
	if payload.ExportedAt <= 0 {
		return errors.New("Missing export time.")
	}
	if len(payload.ModelName) > 200 || strings.ContainsAny(payload.ModelName, `/\:`) {
		return errors.New("Reports cannot include local filesystem paths.")
	}
	refs := []character.Ref{payload.Request.Character.Base}
	for _, item := range payload.Request.Character.AttachItems {
		refs = append(refs, item.Path)
	}
	if mount := payload.Request.Character.Mount; mount != nil {
		refs = append(refs, mount.Path)
	}
	for _, ref := range refs {
		if ref.Type != "wowhead" && ref.Type != "local" && ref.Type != "displayID" {
			return errors.New("Invalid model reference type.")
		}
		if len(ref.Value) > 2048 {
			return errors.New("Invalid model reference.")
		}
		if ref.Type == "wowhead" && !validWowheadURL(ref.Value) {
			return errors.New("Invalid Wowhead reference.")
		}
		if ref.Type == "local" {
			value := strings.ReplaceAll(ref.Value, `\`, "/")
			if strings.Contains(value, ":") || strings.HasPrefix(value, "/") || strings.Contains(value, "..") {
				return errors.New("Reports cannot include local filesystem paths. Use WoW asset references instead.")
			}
		}
		if ref.Type == "displayID" && strings.ContainsAny(ref.Value, `/\:`) {
			return errors.New("Reports cannot include local filesystem paths.")
		}
	}
	if name := payload.Request.OutputFileName; len(name) > 200 || strings.ContainsAny(name, `/\:`) {
		return errors.New("Reports cannot include local filesystem paths.")
	}
	return nil
}
