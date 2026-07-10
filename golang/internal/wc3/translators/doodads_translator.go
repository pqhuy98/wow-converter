package translators

import (
	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

type doodadFlag byte

const (
	doodadFlagUndefined         doodadFlag = 0
	doodadFlagVisible           doodadFlag = 1
	doodadFlagSolid             doodadFlag = 2
	doodadFlagSolidCustomHeight doodadFlag = 6
)

// DoodadsTranslator handles war3map.doo.
type DoodadsTranslator struct{}

// JSONToWar serializes doodads to binary.
func (DoodadsTranslator) JSONToWar(composite data.DoodadList) wc3.WarResult {
	out := wc3.NewHexBufferWriter()
	out.AddChars("W3do")
	out.AddInt(8)
	out.AddInt(11)
	out.AddInt(len(composite.Doodads))

	for _, tree := range composite.Doodads {
		out.AddChars(tree.Type)
		out.AddInt(tree.Variation)
		out.AddFloat(tree.Position[0])
		out.AddFloat(tree.Position[1])
		out.AddFloat(tree.Position[2])
		out.AddFloat(wc3.Deg2Rad(float32(tree.Angle)))

		scale := tree.Scale
		if scale == [3]float32{} {
			scale = [3]float32{1, 1, 1}
		}
		out.AddFloat(scale[0])
		out.AddFloat(scale[1])
		out.AddFloat(scale[2])
		out.AddChars(tree.SkinID)

		treeFlag := byte(doodadFlagSolid)
		if tree.Flags.CustomHeight {
			treeFlag = byte(doodadFlagSolidCustomHeight)
		} else if !tree.Flags.Visible && !tree.Flags.Solid {
			treeFlag = byte(doodadFlagUndefined)
		} else if tree.Flags.Visible && !tree.Flags.Solid {
			treeFlag = byte(doodadFlagVisible)
		}
		out.AddByte(treeFlag)

		life := byte(tree.Life)
		if tree.Life == 0 {
			life = 100
		}
		out.AddByte(life)
		out.AddInt(tree.RandomItemSetPtr)
		out.AddInt(len(tree.DroppedItemSets))
		for _, itemSet := range tree.DroppedItemSets {
			out.AddInt(len(itemSet.Items))
			for _, item := range itemSet.Items {
				out.AddChars(item.ItemID)
				out.AddInt(int(item.Chance))
			}
		}
		out.AddInt(tree.ID)
	}

	out.AddInt(0)
	out.AddInt(len(composite.SpecialDoodads))
	for _, special := range composite.SpecialDoodads {
		out.AddChars(special.Type)
		out.AddInt(int(special.Position[0]))
		out.AddInt(int(special.Position[1]))
		out.AddInt(int(special.Position[2]))
	}
	return wc3.WarResult{Buffer: out.GetBuffer()}
}

// WarToJSON parses war3map.doo bytes.
func (DoodadsTranslator) WarToJSON(buffer []byte) wc3.JsonResult[data.DoodadList] {
	result := []data.Doodad{}
	buf := wc3.NewW3Buffer(buffer)

	buf.ReadChars(4)
	buf.ReadInt()
	buf.ReadInt()
	numDoodads := int(buf.ReadInt())

	for i := 0; i < numDoodads; i++ {
		doodad := data.Doodad{
			Angle:    -1,
			Life:     -1,
			ID:       -1,
			Flags:    data.DoodadFlag{Visible: true, Solid: true},
			Scale:    [3]float32{0, 0, 0},
			Position: [3]float32{0, 0, 0},
		}
		doodad.Type = buf.ReadChars(4)
		doodad.Variation = int(buf.ReadInt())
		doodad.Position = [3]float32{buf.ReadFloat(), buf.ReadFloat(), buf.ReadFloat()}
		doodad.Angle = wc3.Angle(wc3.Rad2Deg(buf.ReadFloat()))
		doodad.Scale = [3]float32{buf.ReadFloat(), buf.ReadFloat(), buf.ReadFloat()}
		doodad.SkinID = buf.ReadChars(4)

		flags := doodadFlag(buf.ReadByte())
		doodad.Flags = data.DoodadFlag{
			Visible:      flags == doodadFlagVisible || flags == doodadFlagSolid || flags == doodadFlagSolidCustomHeight,
			Solid:        flags == doodadFlagSolid || flags == doodadFlagSolidCustomHeight,
			CustomHeight: flags == doodadFlagSolidCustomHeight,
		}
		doodad.Life = int(buf.ReadByte())
		doodad.RandomItemSetPtr = int(buf.ReadInt())
		numberOfItemSets := int(buf.ReadInt())
		for j := 0; j < numberOfItemSets; j++ {
			numberOfItems := int(buf.ReadInt())
			itemSet := data.ItemSet{}
			for k := 0; k < numberOfItems; k++ {
				itemSet.Items = append(itemSet.Items, data.DroppableItem{
					ItemID: buf.ReadChars(4),
					Chance: buf.ReadInt(),
				})
			}
			doodad.DroppedItemSets = append(doodad.DroppedItemSets, itemSet)
		}
		doodad.ID = int(buf.ReadInt())
		result = append(result, doodad)
	}

	special := []data.SpecialDoodad{}
	buf.ReadInt()
	numSpecialDoodads := int(buf.ReadInt())
	for i := 0; i < numSpecialDoodads; i++ {
		special = append(special, data.SpecialDoodad{
			Type: buf.ReadChars(4),
			Position: [3]float32{
				float32(buf.ReadInt()),
				float32(buf.ReadInt()),
				float32(buf.ReadInt()),
			},
		})
	}

	return wc3.JsonResult[data.DoodadList]{JSON: data.DoodadList{
		Doodads:        result,
		SpecialDoodads: special,
	}}
}
