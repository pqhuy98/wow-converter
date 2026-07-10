package translators

import (
	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

// UnitsTranslator handles war3mapUnits.doo.
type UnitsTranslator struct{}

// JSONToWar serializes units to binary.
func (UnitsTranslator) JSONToWar(units []data.Unit) wc3.WarResult {
	return jsonToWarUnits(units)
}

// WarToJSON parses war3mapUnits.doo bytes.
func (UnitsTranslator) WarToJSON(buffer []byte) wc3.JsonResult[[]data.Unit] {
	return warToJSONUnits(buffer)
}

func jsonToWarUnits(unitsJson []data.Unit) wc3.WarResult {
	out := wc3.NewHexBufferWriter()

	out.AddChars("W3do")
	out.AddInt(9)
	out.AddInt(11)
	out.AddInt(len(unitsJson))

	for i := range unitsJson {
		unit := unitsJson[i]

		out.AddChars(unit.Type)
		out.AddInt(unit.Variation)
		out.AddFloat(unit.Position[0])
		out.AddFloat(unit.Position[1])
		out.AddFloat(unit.Position[2])
		out.AddFloat(unit.Rotation)

		scale := unit.Scale
		if scale == [3]float32{} {
			scale = [3]float32{1, 1, 1}
		}
		out.AddFloat(scale[0])
		out.AddFloat(scale[1])
		out.AddFloat(scale[2])

		out.AddChars(unit.Skin)
		out.AddByte(0)
		out.AddInt(unit.Player)
		out.AddByte(0)
		out.AddByte(0)
		out.AddInt(unit.Hitpoints)
		out.AddInt(unit.Mana)
		out.AddInt(unit.RandomItemSetPtr)
		out.AddInt(len(unit.DroppedItemSets))
		for _, itemSet := range unit.DroppedItemSets {
			out.AddInt(len(itemSet.Items))
			for _, item := range itemSet.Items {
				out.AddChars(item.ItemID)
				out.AddInt(int(item.Chance))
			}
		}

		out.AddInt(unit.Gold)
		out.AddFloat(unit.TargetAcquisition)

		hero := unit.Hero
		if hero == (data.UnitHero{}) {
			hero = data.UnitHero{Level: 1, Str: 1, Agi: 1, Int: 1}
		}
		out.AddInt(hero.Level)
		out.AddInt(hero.Str)
		out.AddInt(hero.Agi)
		out.AddInt(hero.Int)

		out.AddInt(len(unit.Inventory))
		for _, item := range unit.Inventory {
			out.AddInt(item.Slot - 1)
			out.AddChars(item.Type)
		}

		out.AddInt(len(unit.Abilities))
		for _, ability := range unit.Abilities {
			out.AddChars(ability.Ability)
			active := 0
			if ability.Active {
				active = 1
			}
			out.AddInt(active)
			out.AddInt(ability.Level)
		}

		out.AddInt(unit.Random.Type)
		switch unit.Random.Type {
		case 0:
			out.AddByte(byte(unit.Random.Level))
			out.AddByte(0)
			out.AddByte(0)
			out.AddByte(byte(unit.Random.ItemClass))
		case 1:
			out.AddInt(unit.Random.GroupIndex)
			out.AddInt(unit.Random.ColumnIndex)
		case 2:
			out.AddInt(len(unit.Random.UnitSet))
			for _, spawnableUnit := range unit.Random.UnitSet {
				out.AddChars(spawnableUnit.UnitID)
				out.AddInt(spawnableUnit.Chance)
			}
		}

		color := unit.Color
		if color == 0 {
			color = unit.Player
		}
		out.AddInt(color)
		out.AddInt(unit.Waygate)
		out.AddInt(unit.ID)
	}

	return wc3.WarResult{Buffer: out.GetBuffer()}
}

func warToJSONUnits(buffer []byte) wc3.JsonResult[[]data.Unit] {
	result := []data.Unit{}
	buf := wc3.NewW3Buffer(buffer)

	buf.ReadChars(4)
	fileVersion := buf.ReadInt()
	subVersion := buf.ReadInt()
	numUnits := int(buf.ReadInt())

	for i := 0; i < numUnits; i++ {
		unit := data.Unit{
			Variation:        -1,
			Hitpoints:        -1,
			Mana:             -1,
			RandomItemSetPtr: -1,
			Color:            -1,
			Waygate:          -1,
			ID:               -1,
			Hero:             data.UnitHero{Level: 1, Str: 1, Agi: 1, Int: 1},
			Random:           data.UnitRandom{Type: -1},
			Scale:            [3]float32{0, 0, 0},
			Position:         [3]float32{0, 0, 0},
		}

		unit.Type = buf.ReadChars(4)
		unit.Variation = int(buf.ReadInt())
		unit.Position = [3]float32{buf.ReadFloat(), buf.ReadFloat(), buf.ReadFloat()}
		unit.Rotation = buf.ReadFloat()
		unit.Scale = [3]float32{buf.ReadFloat(), buf.ReadFloat(), buf.ReadFloat()}

		if fileVersion > 7 {
			unit.Skin = buf.ReadChars(4)
		} else {
			unit.Skin = unit.Type
		}

		buf.ReadByte()
		unit.Player = int(buf.ReadInt())
		buf.ReadByte()
		buf.ReadByte()
		unit.Hitpoints = int(buf.ReadInt())
		unit.Mana = int(buf.ReadInt())

		if subVersion != 9 {
			unit.RandomItemSetPtr = int(buf.ReadInt())
		}
		numDroppedItemSets := int(buf.ReadInt())
		for j := 0; j < numDroppedItemSets; j++ {
			itemSet := data.ItemSet{}
			numDroppableItems := int(buf.ReadInt())
			for k := 0; k < numDroppableItems; k++ {
				itemSet.Items = append(itemSet.Items, data.DroppableItem{
					ItemID: buf.ReadChars(4),
					Chance: buf.ReadInt(),
				})
			}
			unit.DroppedItemSets = append(unit.DroppedItemSets, itemSet)
		}

		unit.Gold = int(buf.ReadInt())
		unit.TargetAcquisition = buf.ReadFloat()

		unit.Hero.Level = int(buf.ReadInt())
		if subVersion != 9 {
			unit.Hero.Str = int(buf.ReadInt())
			unit.Hero.Agi = int(buf.ReadInt())
			unit.Hero.Int = int(buf.ReadInt())
		}

		numItemsInventory := int(buf.ReadInt())
		for j := 0; j < numItemsInventory; j++ {
			unit.Inventory = append(unit.Inventory, data.UnitInventory{
				Slot: int(buf.ReadInt()) + 1,
				Type: buf.ReadChars(4),
			})
		}

		numModifiedAbil := int(buf.ReadInt())
		for j := 0; j < numModifiedAbil; j++ {
			unit.Abilities = append(unit.Abilities, data.UnitAbility{
				Ability: buf.ReadChars(4),
				Active:  buf.ReadInt() == 1,
				Level:   int(buf.ReadInt()),
			})
		}

		unit.Random.Type = int(buf.ReadInt())
		switch unit.Random.Type {
		case 0:
			unit.Random.Level = int(buf.ReadByte())
			buf.ReadByte()
			buf.ReadByte()
			unit.Random.ItemClass = int(buf.ReadByte())
		case 1:
			unit.Random.GroupIndex = int(buf.ReadInt())
			unit.Random.ColumnIndex = int(buf.ReadInt())
		case 2:
			numDiffAvailUnits := int(buf.ReadInt())
			for k := 0; k < numDiffAvailUnits; k++ {
				unit.Random.UnitSet = append(unit.Random.UnitSet, data.UnitSetEntry{
					UnitID: buf.ReadChars(4),
					Chance: int(buf.ReadInt()),
				})
			}
		}

		unit.Color = int(buf.ReadInt())
		unit.Waygate = int(buf.ReadInt())
		unit.ID = int(buf.ReadInt())
		result = append(result, unit)
	}

	return wc3.JsonResult[[]data.Unit]{JSON: result}
}
