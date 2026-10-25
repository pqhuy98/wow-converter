package creature

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

// RefreshShadowSkinOverrides updates editor-saved shadow dimensions that would
// otherwise override regenerated unit fields. Other skin data stays byte-identical.
func RefreshShadowSkinOverrides(outputDir string, mm *extra.MapManager) error {
	updates := map[string]map[string]float32{}
	for _, unit := range mm.UnitTypes {
		for _, field := range unit.Data {
			switch field.ID {
			case "ushw", "ushh", "ushx", "ushy":
				if value, ok := field.Value.(float32); ok {
					if updates[unit.Code] == nil {
						updates[unit.Code] = map[string]float32{}
					}
					updates[unit.Code][field.ID] = value
				}
			}
		}
	}
	if len(updates) == 0 {
		return nil
	}
	path := filepath.Join(outputDir, "war3mapSkin.w3u")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	changed, err := patchShadowSkinDimensions(raw, updates)
	if err != nil {
		return fmt.Errorf("refresh unit shadow skins: %w", err)
	}
	if !changed {
		return nil
	}
	return os.WriteFile(path, raw, 0o644)
}

func patchShadowSkinDimensions(raw []byte, updates map[string]map[string]float32) (bool, error) {
	// The object-table serializer normalizes to version 2. Walk the unit skin
	// format instead, retaining version-3 sets/flags and every unrelated byte.
	r := bytes.NewReader(raw)
	readInt := func() (uint32, error) {
		var value uint32
		err := binary.Read(r, binary.LittleEndian, &value)
		return value, err
	}
	version, err := readInt()
	if err != nil || version < 1 || version > 3 {
		return false, fmt.Errorf("unsupported/truncated unit skin version %d", version)
	}
	changed := false
	for table := 0; table < 2; table++ {
		count, err := readInt()
		if err != nil {
			return false, err
		}
		for object := uint32(0); object < count; object++ {
			var ids [8]byte
			if _, err := io.ReadFull(r, ids[:]); err != nil {
				return false, err
			}
			sets := uint32(1)
			if version == 3 {
				if sets, err = readInt(); err != nil {
					return false, err
				}
			}
			for set := uint32(0); set < sets; set++ {
				if version == 3 {
					if _, err := readInt(); err != nil { // Preserve the set flag.
						return false, err
					}
				}
				fields, err := readInt()
				if err != nil {
					return false, err
				}
				for field := uint32(0); field < fields; field++ {
					var header [8]byte
					if _, err := io.ReadFull(r, header[:]); err != nil {
						return false, err
					}
					kind := binary.LittleEndian.Uint32(header[4:])
					if kind == 3 {
						for {
							b, err := r.ReadByte()
							if err != nil {
								return false, err
							}
							if b == 0 {
								break
							}
						}
					} else if kind <= 2 {
						offset := len(raw) - r.Len()
						bits, err := readInt()
						if err != nil {
							return false, err
						}
						value, ok := updates[string(ids[4:])][string(header[:4])]
						if table == 1 && kind != 0 && ok && bits != math.Float32bits(value) {
							binary.LittleEndian.PutUint32(raw[offset:], math.Float32bits(value))
							changed = true
						}
					} else {
						return false, fmt.Errorf("unknown unit skin field type %d", kind)
					}
					if _, err := readInt(); err != nil { // Field end marker.
						return false, err
					}
				}
			}
		}
	}
	return changed, nil
}
