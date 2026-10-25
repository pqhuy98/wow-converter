package creature

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/server/reportshot"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

type AssetExportResult struct {
	Models    character.CreatureAssets
	Icons     map[string]string
	IconError error
}

// Temporary experiment: combat-capable character NPCs idle in their ready pose.
const combatCharacterStandReady = true

// ExportCreatureAssets queues icons as complete models become available.
// Optional icon errors are returned separately from fatal model/cancel errors.
func ExportCreatureAssets(ctx context.Context, creatures []azerothcore.Creature, outputDir string, cfg config.Config, wowClient client.Client, workers int, destructibles bool, onProgress func(int, int)) (AssetExportResult, error) {
	// NPC texture caps must not overwrite shared scenery imports.
	cfg.AssetPrefix = path.Join(cfg.AssetPrefix, "npcs")
	var result AssetExportResult
	callbacks := character.CreatureExportCallbacks{Progress: onProgress}
	if combatCharacterStandReady && !destructibles {
		callbacks.UseStandReady = creatureCanAttack
	}
	var queue *reportshot.ModelIconQueue
	if !destructibles {
		queue = reportshot.NewModelIconQueue(ctx, outputDir, cfg.OverrideModels)
		callbacks.ModelReady = queue.Enqueue
		callbacks.ModelsDone = queue.Wait
	}
	var err error
	result.Models, err = character.ExportCreatureModels(ctx, creatures, outputDir, cfg, wowClient, workers, callbacks)
	if err != nil {
		return result, err
	}
	if queue != nil {
		seen := map[string]bool{}
		var models []string
		for _, asset := range result.Models {
			if !seen[asset.ModelPath] {
				seen[asset.ModelPath] = true
				models = append(models, asset.ModelPath)
			}
		}
		sort.Strings(models)
		result.Icons, result.IconError = queue.Finish(models)
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func ApplyCreatureAssets(mm *extra.MapManager, units []PlacedUnit, assets character.CreatureAssets, icons map[string]string) {
	byCode := map[string]PlacedUnit{}
	for i := range units {
		u := &units[i]
		asset := assets.For(u.Creature)
		if asset == nil {
			continue
		}
		u.ModelPath = asset.ModelPath
		u.IconPath = icons[asset.ModelPath]
		if u.TypeCode != "" {
			byCode[u.TypeCode] = *u
		}
	}
	set := func(fields *[]data.Modification, id, value string) {
		value = strings.ReplaceAll(value, "/", "\\")
		for i := range *fields {
			if (*fields)[i].ID == id {
				(*fields)[i].Value = value
				(*fields)[i].Type = data.ModificationString
				return
			}
		}
		*fields = append(*fields, data.Modification{ID: id, Type: data.ModificationString, Value: value})
	}
	for i := range mm.UnitTypes {
		unit := &mm.UnitTypes[i]
		source, ok := byCode[unit.Code]
		if !ok {
			continue
		}
		set(&unit.Data, "umdl", source.ModelPath)
		set(&unit.Data, "upor", source.ModelPath)
		if asset := assets.For(source.Creature); asset != nil {
			set(&unit.Data, "urac", creatureRace(source.Creature, asset.WowRace))
		}
		icon := source.IconPath
		if icon == "" {
			icon = `ReplaceableTextures\CommandButtons\BTNFootman.blp`
		}
		set(&unit.Data, "uico", icon)
	}
	for i := range mm.DestructibleTypes {
		object := &mm.DestructibleTypes[i]
		if source, ok := byCode[object.Code]; ok {
			set(&object.Data, "bfil", source.ModelPath)
			set(&object.Data, "bgpm", source.ModelPath)
		}
	}
}
