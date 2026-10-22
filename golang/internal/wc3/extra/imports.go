package extra

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// EnsureCustomImports adds sorted, unique map-root import paths.
func EnsureCustomImports(outputDir string, paths []string) error {
	wanted := map[string]bool{}
	for _, path := range paths {
		wanted[path] = true
	}
	path := filepath.Join(outputDir, "war3map.imp")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(raw) < 8 {
		return fmt.Errorf("truncated map import list")
	}
	count := binary.LittleEndian.Uint32(raw[4:])
	pos := 8
	for i := uint32(0); i < count; i++ {
		if pos >= len(raw) {
			return fmt.Errorf("truncated map import entry")
		}
		flag := raw[pos]
		pos++
		end := bytes.IndexByte(raw[pos:], 0)
		if end < 0 {
			return fmt.Errorf("unterminated map import path")
		}
		if flag&0xf == 13 {
			delete(wanted, string(raw[pos:pos+end]))
		}
		pos += end + 1
	}
	missing := make([]string, 0, len(wanted))
	for path := range wanted {
		missing = append(missing, path)
	}
	sort.Strings(missing)
	for _, path := range missing {
		raw = append(raw, 13) // Custom path, relative to the map root.
		raw = append(raw, path...)
		raw = append(raw, 0)
	}
	binary.LittleEndian.PutUint32(raw[4:], count+uint32(len(missing)))
	return os.WriteFile(path, raw, 0o644)
}
