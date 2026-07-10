package writers

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

// GetExportPath returns a path under the configured export directory.
func GetExportPath(file string) string {
	cfg := server.GetConfig()
	normalized := file
	if cfg.RemovePathSpaces {
		normalized = strings.ReplaceAll(file, " ", "")
	}
	return filepath.Clean(filepath.Join(cfg.ExportDirectory, normalized))
}

// ReplaceFile takes the directory from fileA and combines it with the basename of fileB.
func ReplaceFile(fileA, fileB string) string {
	return filepath.Join(filepath.Dir(fileA), filepath.Base(fileB))
}

// ReplaceExtension replaces a file extension.
func ReplaceExtension(file, ext string) string {
	return filepath.Join(filepath.Dir(file), strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))+ext)
}

// Win32ToPosix converts backslashes to forward slashes.
func Win32ToPosix(str string) string {
	return strings.ReplaceAll(str, "\\", "/")
}

// WriteOutputFile writes export artifact bytes to disk.
func WriteOutputFile(filePath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0o644)
}

// OutputFileExists reports whether an export file exists.
func OutputFileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}
