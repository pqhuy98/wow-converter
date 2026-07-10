package convertlog

import (
	"log"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/config"
)

// Loading logs a bundle file load message when not in bulk export mode.
func Loading(cfg config.Config, path string) {
	if cfg.IsBulkExport {
		return
	}
	log.Printf("Loading: %s", ansi.Gray(path))
}

// MapGenerate logs a cyan map-generate phase label.
func MapGenerate(label string) {
	log.Printf("%s", ansi.Cyanf("[map-generate] %s", label))
}
