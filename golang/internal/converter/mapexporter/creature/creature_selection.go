package creature

import (
	"context"
	"fmt"
	"math"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func PrepareCreatureSelection(ctx context.Context, wowClient client.Client, manager *common.WowObjectManager) error {
	resolver, ok := wowClient.(client.NpcSelectionClient)
	if !ok {
		return nil
	}
	type key struct {
		display, family, level int
		scale                  float64
	}
	radii := map[key]float64{}
	var failure error
	manager.IterateObjects(func(obj *common.WowObject, _ common.ObjectAbsolute) {
		if failure != nil {
			return
		}
		if err := ctx.Err(); err != nil {
			failure = err
			return
		}
		c := common.ObjectCreature(obj)
		if c == nil || !common.IsWowUnit(obj) {
			return
		}
		family := 0
		if c.Gameplay != nil {
			family = c.Gameplay.Family
		}
		scale := c.Model.DisplayScale
		if scale <= 0 {
			scale = 1
		}
		k := key{c.Model.CreatureDisplayID, family, c.Template.MaxLevel, scale}
		radius, found := radii[k]
		if !found {
			var err error
			radius, err = resolver.ResolveNpcSelection(ctx, k.display, casc.NpcSelectionOptions{Family: family, Level: k.level, ObjectScale: scale})
			if err != nil {
				failure = fmt.Errorf("NPC %s (%d) selection radius: %w", c.Template.Name, k.display, err)
				return
			}
			if radius <= 0 || math.IsNaN(radius) || math.IsInf(radius, 0) {
				failure = fmt.Errorf("invalid native selection radius for NPC display %d", k.display)
				return
			}
			radii[k] = radius
		}
		c.Model.SelectionRadius = &radius
	})
	return failure
}

func creatureSelectionScale(c azerothcore.Creature, yardScale float64) float64 {
	radius := 1.2 // The WoW client's own fallback for an empty/missing stand box.
	if c.Model.SelectionRadius != nil {
		radius = *c.Model.SelectionRadius
	}
	// Radius already includes WoW's nonlinear native unit/display/family scale.
	// yardScale includes raw yards->model conversion, map fit and export unit size.
	// WC3 UI/MiscData SelectionCircle.ScaleFactor=72 is the circle's diameter.
	return 2 * radius * yardScale / 72
}
