package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

type mapUnitEntry struct {
	Name string
	MDL  *mdl.MDL
}

func deathTimeSec(m *mdl.MDL) float64 {
	if m == nil {
		return 6
	}
	for _, seq := range m.Sequences {
		if strings.HasPrefix(seq.Name, "Death") {
			return float64(seq.Interval[1]-seq.Interval[0]) / 1000
		}
	}
	return 6
}

func populateRegressionMapUnits(mapDir, format string, entries []mapUnitEntry) error {
	if len(entries) == 0 {
		return nil
	}

	mapMgr := extra.NewMapManager()
	if err := mapMgr.Load(mapDir); err != nil {
		return fmt.Errorf("map load: %w", err)
	}

	filtered := mapMgr.Units[:0]
	for _, u := range mapMgr.Units {
		if _, ok := u.Type.(string); ok {
			filtered = append(filtered, u)
		}
	}
	mapMgr.Units = filtered
	mapMgr.UnitTypes = nil

	terrain := mapMgr.GetTerrain()
	padding := float64(10 * config.DistancePerTile)
	width := float64(terrain.Map.Width*config.DistancePerTile) - 2*padding
	offsetX := float64(terrain.Map.Offset.X)
	offsetY := float64(terrain.Map.Offset.Y)

	for i, entry := range entries {
		name := entry.Name
		modelFile := name + "." + format
		unitType := mapMgr.AddUnitType(true, "Hpal", []data.Modification{
			{ID: "unam", Type: data.ModificationString, Value: name},
			{ID: "upro", Type: data.ModificationString, Value: name},
			{ID: "umdl", Type: data.ModificationString, Value: modelFile},
			{ID: "usca", Type: data.ModificationReal, Value: float64(1)},
			{ID: "ussc", Type: data.ModificationReal, Value: float64(2)},
			{ID: "ua1b", Type: data.ModificationInt, Value: int32(500)},
			{ID: "uabi", Type: data.ModificationString, Value: "A003,A001,A002,A000"},
			{ID: "udtm", Type: data.ModificationUnreal, Value: deathTimeSec(entry.MDL)},
		})

		i2 := float64(i * 500)
		position := [3]float32{
			float32(math.Mod(i2, width) + padding + offsetX),
			float32(-(math.Floor(i2/width)*1000 + padding + offsetY)),
			0,
		}
		log.Printf("%s at location %.0f, %.0f", name, position[0], position[1])

		mapMgr.AddUnit(unitType, data.Unit{
			Variation:         0,
			Position:          position,
			Rotation:          270,
			Scale:             [3]float32{1, 1, 1},
			Skin:              unitType.Code,
			Player:            0,
			Hitpoints:         100,
			Mana:              0,
			RandomItemSetPtr:  -1,
			DroppedItemSets:   nil,
			Gold:              0,
			TargetAcquisition: -1,
			Hero:              data.UnitHero{Level: 10},
			Inventory:         nil,
			Abilities:         nil,
			Random:            data.UnitRandom{},
			Color:             23,
			Waygate:           -1,
			ID:                0,
		})
	}

	log.Printf("Unit counts: %d", len(mapMgr.Units))
	log.Printf("Unit type counts: %d", len(mapMgr.UnitTypes))

	if err := mapMgr.Save(mapDir); err != nil {
		return fmt.Errorf("map save: %w", err)
	}
	for _, file := range []string{"war3map.j", "war3map.imp", "war3map.wts", "war3mapSkin.w3u"} {
		if err := os.Remove(filepath.Join(mapDir, file)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", file, err)
		}
	}
	log.Printf("Map saved to %s", mapDir)
	return nil
}
