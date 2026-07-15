package mapexporter

import (
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestPurgeKeepsUsedModelsByName(t *testing.T) {
	cfg := config.DefaultConfig()
	am := common.NewAssetManager(cfg, nil, nil)

	objectPath := "world/maps/azeroth/elwynn/foo"
	modelName := cfg.AssetPrefix + "/" + objectPath
	m := mdl.New(mdl.NewMDLOptions{Name: modelName})
	am.Models()[objectPath] = &common.Model{RelativePath: modelName, MDL: m}

	used := map[string]struct{}{modelName: {}}
	for k, model := range am.Models() {
		rel := strings.ReplaceAll(model.MDL.Model.Name, "\\", "/")
		if _, ok := used[rel]; !ok {
			delete(am.Models(), k)
		}
	}
	if len(am.Models()) != 1 {
		t.Fatalf("expected 1 model after purge, got %d", len(am.Models()))
	}
}

func TestAddDoodadTypeMutationsPersist(t *testing.T) {
	mm := extra.NewMapManager()
	dt := mm.AddDoodadType(nil, false)
	dt.Data = append(dt.Data, data.Modification{ID: "dfil", Type: data.ModificationString, Value: "wow/foo"})
	if len(mm.DoodadTypes) != 1 {
		t.Fatalf("expected 1 doodad type, got %d", len(mm.DoodadTypes))
	}
	if len(mm.DoodadTypes[0].Data) != 1 {
		t.Fatalf("expected persisted dfil mod on manager entry, got %d mods", len(mm.DoodadTypes[0].Data))
	}
}

func TestPurgeMatchesDfilFromDoodadType(t *testing.T) {
	cfg := config.DefaultConfig()
	am := common.NewAssetManager(cfg, nil, nil)

	objectPath := "world/maps/azeroth/elwynn/bar"
	modelName := cfg.AssetPrefix + "/" + objectPath
	m := mdl.New(mdl.NewMDLOptions{Name: modelName})
	am.Models()[objectPath] = &common.Model{RelativePath: modelName, MDL: m}

	mm := extra.NewMapManager()
	dt := mm.AddDoodadType(nil, false)
	dt.Data = append(dt.Data,
		data.Modification{ID: "dfil", Type: data.ModificationString, Level: 0, Column: 0, Value: modelName},
		data.Modification{ID: "dnam", Type: data.ModificationString, Level: 0, Column: 0, Value: "~D bar -- m2 -- " + dt.Code},
	)

	usedModelPaths := map[string]struct{}{}
	for _, t := range mm.DoodadTypes {
		for _, mod := range t.Data {
			if (mod.ID == "dfil" || mod.ID == "bfil") && mod.Type == data.ModificationString {
				if s, ok := mod.Value.(string); ok {
					usedModelPaths[strings.ReplaceAll(s, "\\", "/")] = struct{}{}
				}
			}
		}
	}
	for k, model := range am.Models() {
		rel := strings.ReplaceAll(model.MDL.Model.Name, "\\", "/")
		if _, ok := usedModelPaths[rel]; !ok {
			delete(am.Models(), k)
		}
	}
	if len(am.Models()) != 1 {
		t.Fatalf("expected 1 model, got %d; used=%v name=%q", len(am.Models()), usedModelPaths, modelName)
	}
}
