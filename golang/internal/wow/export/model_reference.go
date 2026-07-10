package export

import (
	"path/filepath"
	"regexp"
	"strings"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
)

var modelRefExtPattern = regexp.MustCompile(`(?i)\.(obj|m2|wmo)$`)

// ModelReferencePath returns a listfile-style model path for CSV placement.
func ModelReferencePath(fileDataID uint32, kind string, wmoSet *int) string {
	fileName, ok := archivecasc.GetByID(int(fileDataID))
	if !ok {
		ext := ".wmo"
		if kind == "m2" {
			ext = ".m2"
		}
		return archivecasc.FormatUnknownFile(int(fileDataID), ext)
	}
	if kind == "wmo" && wmoSet != nil {
		return writers.ReplaceExtension(fileName, "_set"+itoa(*wmoSet)+".wmo")
	}
	if kind == "m2" && !strings.HasSuffix(strings.ToLower(fileName), ".m2") {
		return writers.ReplaceExtension(fileName, ".m2")
	}
	return fileName
}

// PlacementCsvPath returns the interior/placement CSV path for a model reference.
func PlacementCsvPath(modelPath string) string {
	return modelRefExtPattern.ReplaceAllString(modelPath, "_ModelPlacementInformation.csv")
}

// ResolveModelStoragePath resolves on-disk path for a shared or tile-local model reference.
func ResolveModelStoragePath(fileName, tileDir string, enableSharedChildren bool) string {
	if enableSharedChildren {
		return writers.GetExportPath(fileName)
	}
	return filepath.Join(tileDir, filepath.Base(fileName))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
