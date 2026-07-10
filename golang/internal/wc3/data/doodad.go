package data

import "github.com/pqhuy98/wow-converter/internal/wc3"

// DoodadFlag mirrors WC3 doodad instance flags.
type DoodadFlag struct {
	Visible      bool
	Solid        bool
	CustomHeight bool
}

// Doodad is a placed doodad/destructible instance.
type Doodad struct {
	Type             string
	Variation        int
	Position         [3]float32
	Angle            wc3.Angle
	Scale            [3]float32
	SkinID           string
	Flags            DoodadFlag
	Life             int
	RandomItemSetPtr int
	DroppedItemSets  []ItemSet
	ID               int
}

// SpecialDoodad is a script-placed special doodad marker.
type SpecialDoodad struct {
	Type     string
	Position [3]float32
}

// DoodadList is [doodads, specialDoodads].
type DoodadList struct {
	Doodads        []Doodad
	SpecialDoodads []SpecialDoodad
}
