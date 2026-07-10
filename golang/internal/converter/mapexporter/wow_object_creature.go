package mapexporter

import (
	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
)

func wowObjectCreature(obj *common.WowObject) *azerothcore.Creature {
	if obj == nil || obj.Creature == nil {
		return nil
	}
	c, _ := obj.Creature.(*azerothcore.Creature)
	return c
}
