package mapexporter

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// The bundled Wrath display index resolves base models without exporting assets
// or querying a client build whose model filenames may differ from AzerothCore.
type creatureVoices map[int]map[string]string

var loadCreatureVoices = sync.OnceValues(func() (creatureVoices, error) {
	path := filepath.Join(workspace.AppRoot(), "resources", "npc-voices.json")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("NPC voices: %w", err)
	}
	defer f.Close()
	voices, err := decodeCreatureVoices(f)
	if err != nil {
		return nil, fmt.Errorf("NPC voices: %w", err)
	}
	return voices, nil
})

func decodeCreatureVoices(r io.Reader) (creatureVoices, error) {
	var resource struct {
		Version int      `json:"version"`
		Pairs   int      `json:"pairs"`
		Sounds  []string `json:"sounds"`
		Models  []struct {
			Path     string         `json:"path"`
			Displays []int          `json:"displays"`
			Voices   map[string]int `json:"voices"`
		} `json:"models"`
	}
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&resource); err != nil {
		return nil, err
	}
	// Reject trailing JSON.
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, fmt.Errorf("invalid NPC voice resource trailer: %v", err)
	}
	if resource.Version != 1 || len(resource.Sounds) == 0 || resource.Sounds[0] != "" || resource.Pairs <= 0 {
		return nil, fmt.Errorf("invalid NPC voice resource header")
	}
	voices := creatureVoices{}
	paths := map[string]bool{}
	pairs := 0
	for _, model := range resource.Models {
		if model.Path == "" || paths[model.Path] || len(model.Displays) == 0 || len(model.Voices) == 0 {
			return nil, fmt.Errorf("invalid/duplicate NPC voice model %q", model.Path)
		}
		paths[model.Path] = true
		names := make(map[string]string, len(model.Voices))
		for name, index := range model.Voices {
			if index < 0 || index >= len(resource.Sounds) {
				return nil, fmt.Errorf("invalid NPC voice index for %q", name)
			}
			names[name] = resource.Sounds[index]
			pairs++
		}
		for _, display := range model.Displays {
			if display <= 0 || voices[display] != nil {
				return nil, fmt.Errorf("invalid/duplicate NPC voice display %d", display)
			}
			voices[display] = names
		}
	}
	if pairs != resource.Pairs {
		return nil, fmt.Errorf("NPC voice pair count: got %d, expected %d", pairs, resource.Pairs)
	}
	return voices, nil
}

func (v creatureVoices) lookup(c azerothcore.Creature) (string, bool) {
	if c.Template.FlagsExtra&0x80 != 0 {
		return "", true // Trigger proxies remain silent even when sharing a real NPC's pair.
	}
	sound, found := v[c.Model.CreatureDisplayID][c.Template.Name]
	return sound, found
}

func applyCreatureVoices(mm *extra.MapManager, units []PlacedUnit, voices creatureVoices) {
	byCode := map[string]azerothcore.Creature{}
	for _, u := range units {
		if u.TypeCode != "" {
			byCode[u.TypeCode] = u.Creature
		}
	}
	matched, missing := 0, 0
	for i := range mm.UnitTypes {
		unit := &mm.UnitTypes[i]
		c, ok := byCode[unit.Code]
		if !ok {
			continue
		}
		sound, found := voices.lookup(c)
		if found {
			matched++
		} else {
			missing++
		}
		// Explicit silence and unknown pairs must not inherit the hfoo parent voice.
		field := data.Modification{ID: "usnd", Type: data.ModificationString, Value: sound}
		replaced := false
		for j := range unit.Data {
			if unit.Data[j].ID == "usnd" {
				unit.Data[j] = field
				replaced = true
				break
			}
		}
		if !replaced {
			unit.Data = append(unit.Data, field)
		}
	}
	log.Printf("NPC voices: %d mapped unit types, %d unknown pairs left silent", matched, missing)
}
