package metadata

import (
	"encoding/json"
	"os"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/convertlog"
)

// UnmarshalJSON is used only for companion metadata, never direct conversion.
func (track *Track) UnmarshalJSON(data []byte) error {
	type diskTrack Track
	parsed := diskTrack{GlobalSeq: uint16(config.BlizzardNull)}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*track = Track(parsed)
	return nil
}

// UnmarshalJSON gives absent companion color tracks the no-global default.
func (color *Color) UnmarshalJSON(data []byte) error {
	type diskColor Color
	parsed := diskColor{
		Color: Track{GlobalSeq: uint16(config.BlizzardNull)},
		Alpha: Track{GlobalSeq: uint16(config.BlizzardNull)},
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*color = Color(parsed)
	return nil
}

// UnmarshalJSON gives absent companion UV tracks the no-global default.
func (transform *TextureTransform) UnmarshalJSON(data []byte) error {
	type diskTransform TextureTransform
	parsed := diskTransform{
		Translation: Track{GlobalSeq: uint16(config.BlizzardNull)},
		Rotation:    Track{GlobalSeq: uint16(config.BlizzardNull)},
		Scaling:     Track{GlobalSeq: uint16(config.BlizzardNull)},
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	*transform = TextureTransform(parsed)
	return nil
}

// Parse reads companion .json metadata from disk when present.
func (f *File) Parse() error {
	convertlog.Loading(f.Config, f.FilePath)
	data, err := os.ReadFile(f.FilePath)
	if err != nil {
		f.IsLoaded = false
		return nil
	}
	// Materials share a JSON field name across M2 and WMO, but their native
	// contracts differ. Decode that field at this disk boundary only.
	var parsed struct {
		Data
		Materials json.RawMessage `json:"materials"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	if len(parsed.Materials) > 0 {
		if parsed.FileType == "wmo" {
			err = json.Unmarshal(parsed.Materials, &parsed.WMOMaterials)
		} else {
			err = json.Unmarshal(parsed.Materials, &parsed.Data.Materials)
		}
		if err != nil {
			return err
		}
	}
	f.LoadFromData(parsed.Data)
	return nil
}
