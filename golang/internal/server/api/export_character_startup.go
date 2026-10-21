package api

import (
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
)

// StartupDemoRequests are pre-queued exports for shared hosting (mirrors export-character.startup.ts).
func StartupDemoRequests() []exportCharacterRequest {
	return []exportCharacterRequest{{
		Character: character.Character{
			Base:                       character.WowheadRef("https://www.wowhead.com/wotlk/npc=36597/the-lich-king"),
			Size:                       "hero",
			AttackTag:                  animmap.AttackTag2H,
			InGameMovespeed:            270,
			PortraitCameraSequenceName: "Stand",
			AttachItems: map[string]character.AttachItem{
				"1": {Path: character.WowheadRef("https://www.wowhead.com/classic/item=231885/frostmourne")},
			},
		},
		OutputFileName: "demo-lichking",
		Format:         "mdx",
		Optimization: exportCharacterOptimization{
			FreezeHead:                    boolPtr(true),
			SortSequences:                 boolPtr(true),
			AllMaterialsUnshaded:          boolPtr(false),
			RemoveUnusedVertices:          boolPtr(true),
			RemoveUnusedNodes:             boolPtr(true),
			RemoveUnusedMaterialsTextures: boolPtr(true),
		},
	}}
}
