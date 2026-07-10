package workspace

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// FindRepoRoot walks upward from cwd looking for the wow-converter repo root.
func FindRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := wd
	for {
		if isRepoRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return wd
}

func isRepoRoot(dir string) bool {
	if st, err := os.Stat(filepath.Join(dir, "package.json")); err != nil || st.IsDir() {
		return false
	}
	if st, err := os.Stat(filepath.Join(dir, "golang", "go.mod")); err != nil || !st.Mode().IsRegular() {
		return false
	}
	return true
}

// ChdirRepoRoot changes the process working directory to the repo root when found.
func ChdirRepoRoot() (string, error) {
	root := FindRepoRoot()
	if err := os.Chdir(root); err != nil {
		return root, err
	}
	return root, nil
}

// ResolveRepoPath maps a repo-relative path to an absolute path under FindRepoRoot().
func ResolveRepoPath(rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return filepath.Clean(FindRepoRoot())
	}
	if filepath.IsAbs(rel) {
		abs, err := filepath.Abs(rel)
		if err != nil {
			return filepath.Clean(rel)
		}
		return filepath.Clean(abs)
	}
	return filepath.Clean(filepath.Join(FindRepoRoot(), filepath.FromSlash(rel)))
}

// ResolveExportAssetDir returns an absolute export asset directory.
// Relative paths are resolved against the repo root, matching TS path.resolve behavior.
func ResolveExportAssetDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = filepath.Join(FindRepoRoot(), ".cache", "wow-export")
	} else if !filepath.IsAbs(dir) {
		dir = filepath.Join(FindRepoRoot(), filepath.FromSlash(dir))
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Clean(dir)
	}
	return filepath.Clean(abs)
}

// DefaultExportDir returns the export asset directory (WOW_EXPORT_DIR or .cache/wow-export under repo root).
func DefaultExportDir() string {
	if v := strings.TrimSpace(os.Getenv("WOW_EXPORT_DIR")); v != "" {
		return ResolveExportAssetDir(v)
	}
	if v := strings.TrimSpace(os.Getenv("EXPORT_ASSET_DIR")); v != "" {
		return ResolveExportAssetDir(v)
	}
	return ResolveExportAssetDir("")
}

// WowDataServerURL returns the local wow-data-server base URL from environment.
func WowDataServerURL() string {
	if u := strings.TrimSpace(os.Getenv("WOW_DATA_SERVER_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://127.0.0.1:" + wowDataServerPort()
}

func wowDataServerPort() string {
	if port := strings.TrimSpace(os.Getenv("WOW_DATA_SERVER_PORT")); port != "" {
		return port
	}
	return "17753"
}

// PreferWowDataServerPortFromEnv uses WOW_DATA_SERVER_PORT from repo .env for local compare scripts.
// Shell-level WOW_DATA_SERVER_URL often points at a stale port and makes tools look frozen.
func PreferWowDataServerPortFromEnv() string {
	port := wowDataServerPort()
	if f, err := os.Open(".env"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "#") || line == "" {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			if strings.TrimSpace(key) == "WOW_DATA_SERVER_PORT" {
				value = strings.Trim(strings.TrimSpace(value), `"`)
				if value != "" {
					port = value
				}
				break
			}
		}
	}
	return "http://127.0.0.1:" + port
}

// LoadEnvFile loads KEY=VALUE pairs from a dotenv file without overriding existing env vars.
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
	return scanner.Err()
}
