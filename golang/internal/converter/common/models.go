package common

import (
	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/math"
)

// WowObjectType classifies exported WoW placement objects.
type WowObjectType string

const (
	WowObjectADT  WowObjectType = "adt"
	WowObjectWMO  WowObjectType = "wmo"
	WowObjectM2   WowObjectType = "m2"
	WowObjectGobj WowObjectType = "gobj"
	WowObjectUnit WowObjectType = "unit"
)

// IsWowObjectType reports whether s is a known object type.
func IsWowObjectType(s string) bool {
	switch WowObjectType(s) {
	case WowObjectADT, WowObjectWMO, WowObjectM2, WowObjectGobj, WowObjectUnit:
		return true
	default:
		return false
	}
}

// Model holds a converted asset.
type Model struct {
	RelativePath string
	MDL          *mdl.MDL
	TexturePaths []string
}

// ObjectAbsolute is world-space placement for a tree node.
type ObjectAbsolute struct {
	Position    math.Vector3
	Rotation    math.EulerRotation
	ScaleFactor float64
}

// WowObject is a node in the WoW object tree.
type WowObject struct {
	ID          string
	Type        WowObjectType
	FileDataID  int
	Model       *Model
	Position    [3]float64
	Rotation    [3]float64
	ScaleFactor float64
	Children    []*WowObject
	TileX       int
	TileY       int
	Creature    any // *azerothcore.Creature when Type is unit
}

// ObjectCreature returns the creature payload when type is unit.
func ObjectCreature(o *WowObject) *azerothcore.Creature {
	if o == nil || o.Creature == nil {
		return nil
	}
	c, ok := o.Creature.(*azerothcore.Creature)
	if !ok {
		return nil
	}
	return c
}

// IsWowUnit reports unit type.
func IsWowUnit(o *WowObject) bool { return o != nil && o.Type == WowObjectUnit }

// IsWowAdt reports adt type.
func IsWowAdt(o *WowObject) bool { return o != nil && o.Type == WowObjectADT }

// AsAdt returns adt tile coords when type is adt.
func AsAdt(o *WowObject) (tileX, tileY int, ok bool) {
	if !IsWowAdt(o) {
		return 0, 0, false
	}
	return o.TileX, o.TileY, true
}
