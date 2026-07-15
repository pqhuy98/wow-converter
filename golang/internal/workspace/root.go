package workspace

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ExeDir returns the directory containing the running executable.
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// AppRoot is the base directory for app-relative paths (exports, caches, bin/, resources/).
// Desktop bundle: exe dir when build-go-app markers sit beside the binary.
// Dev (dev, dev:split, wow-data-server, CLI): walk up from cwd to package.json + golang/go.mod.
func AppRoot() string {
	return resolveAppRoot(ExeDir(), workingDir())
}

func resolveAppRoot(exeDir, cwd string) string {
	if isShippedBundleAt(exeDir) {
		return exeDir
	}
	dir := cwd
	for {
		if isDevRepoRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd
}

func isShippedBundleAt(root string) bool {
	for _, rel := range []string{
		filepath.Join("webui", "out", "index.html"),
		filepath.Join("resources", "template-empty.w3x"),
		"bin",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			return false
		}
	}
	return true
}

func isDevRepoRoot(dir string) bool {
	if st, err := os.Stat(filepath.Join(dir, "package.json")); err != nil || st.IsDir() {
		return false
	}
	if st, err := os.Stat(filepath.Join(dir, "golang", "go.mod")); err != nil || !st.Mode().IsRegular() {
		return false
	}
	return true
}

func workingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// IsDesktopApp reports whether AppRoot is the shipped desktop bundle (exe dir).
func IsDesktopApp() bool {
	return AppRoot() == ExeDir()
}

// FindRepoRoot is an alias for AppRoot.
func FindRepoRoot() string { return AppRoot() }

// ChdirAppRoot sets cwd to AppRoot.
func ChdirAppRoot() (string, error) {
	root := AppRoot()
	return root, os.Chdir(root)
}

// ChdirRepoRoot is an alias for ChdirAppRoot.
func ChdirRepoRoot() (string, error) { return ChdirAppRoot() }

// ResolveRepoPath maps a repo-relative path to an absolute path under AppRoot().
func ResolveRepoPath(rel string) string {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return filepath.Clean(AppRoot())
	}
	if filepath.IsAbs(rel) {
		abs, err := filepath.Abs(rel)
		if err != nil {
			return filepath.Clean(rel)
		}
		return filepath.Clean(abs)
	}
	return filepath.Clean(filepath.Join(AppRoot(), filepath.FromSlash(rel)))
}

// ResolveExportAssetDir returns an absolute export asset directory.
func ResolveExportAssetDir(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = filepath.Join(AppRoot(), ".cache", "wow-export")
	} else if !filepath.IsAbs(dir) {
		dir = filepath.Join(AppRoot(), filepath.FromSlash(dir))
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Clean(dir)
	}
	return filepath.Clean(abs)
}

// DefaultExportDir returns the export asset directory (WOW_EXPORT_DIR or .cache/wow-export under AppRoot).
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
