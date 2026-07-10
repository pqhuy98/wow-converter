package metadata

import (
	"encoding/json"
	"os"

	"github.com/pqhuy98/wow-converter/internal/converter/convertlog"
)

// Parse reads companion .json metadata from disk when present.
func (f *File) Parse() error {
	convertlog.Loading(f.Config, f.FilePath)
	data, err := os.ReadFile(f.FilePath)
	if err != nil {
		f.IsLoaded = false
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.LoadFromData(raw)
	return nil
}
