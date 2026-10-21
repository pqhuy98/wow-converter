package mapexporter

import (
	"context"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/server/reportshot"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func renderCreatureIcons(ctx context.Context, outputDir string, assets character.CreatureAssets, destructibles, force bool) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if destructibles {
		return nil, nil
	}
	var models []string
	seen := map[string]bool{}
	for _, asset := range assets {
		if !seen[asset.ModelPath] {
			seen[asset.ModelPath] = true
			models = append(models, asset.ModelPath)
		}
	}
	return reportshot.RenderModelIcons(ctx, outputDir, models, force)
}

func applyCreatureAssets(mm *extra.MapManager, units []PlacedUnit, assets character.CreatureAssets, icons map[string]string) {
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
