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
	// AngleRadians retains the original DOO float when Angle has not been edited.
	AngleRadians     *float32
	GroupID          int32
	Unknown1         int32
	Roll             float32
	Pitch            float32
	Lights           []DoodadLight
	State            *byte
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

// DoodadLight holds the per-instance light overrides introduced in DOO v12.
type DoodadLight struct {
	Index              int32
	ShadowCasting      int32
	Color              int32
	Intensity          float32
	ShadowCastingStart float32
	ShadowCastingEnd   float32
	QuadraticFalloff   float32
	LinearFalloff      float32
	Damping            float32
}
