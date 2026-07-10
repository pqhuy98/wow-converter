package data

// ModificationType is an object-data field type.
type ModificationType string

const (
	ModificationInt    ModificationType = "int"
	ModificationReal   ModificationType = "real"
	ModificationUnreal ModificationType = "unreal"
	ModificationString ModificationType = "string"
)

// TableType identifies original vs custom object tables.
type TableType int

const (
	TableOriginal TableType = iota
	TableCustom
)

// ObjectType identifies WC3 object table kinds.
type ObjectType string

const (
	ObjectUnits        ObjectType = "units"
	ObjectItems        ObjectType = "items"
	ObjectDestructables ObjectType = "destructables"
	ObjectDoodads      ObjectType = "doodads"
	ObjectAbilities    ObjectType = "abilities"
	ObjectBuffs        ObjectType = "buffs"
	ObjectUpgrades     ObjectType = "upgrades"
)

// Modification is a single object-data field override.
type Modification struct {
	ID        string
	Type      ModificationType
	Value     any
	Level     int
	Column    int
	Variation int
}

// ObjectModificationTable holds original/custom object modifications.
type ObjectModificationTable struct {
	Original map[string][]Modification
	Custom   map[string][]Modification
}

// NewObjectModificationTable returns an empty table.
func NewObjectModificationTable() ObjectModificationTable {
	return ObjectModificationTable{
		Original: map[string][]Modification{},
		Custom:   map[string][]Modification{},
	}
}
