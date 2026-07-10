package data

// UnitHero holds hero attribute overrides.
type UnitHero struct {
	Level int
	Str   int
	Agi   int
	Int   int
}

// UnitInventory is a single inventory slot override.
type UnitInventory struct {
	Slot int
	Type string
}

// UnitAbility is a modified ability on a unit instance.
type UnitAbility struct {
	Ability string
	Active  bool
	Level   int
}

// UnitRandom configures random unit spawn data.
type UnitRandom struct {
	Type        int
	Level       int
	ItemClass   int
	GroupIndex  int
	ColumnIndex int
	UnitSet     []UnitSetEntry
}

// UnitSetEntry is one weighted unit in a random spawn set.
type UnitSetEntry struct {
	UnitID string
	Chance int
}

// Unit is a placed unit instance.
type Unit struct {
	Type              string
	Variation        int
	Position         [3]float32
	Rotation         float32
	Scale            [3]float32
	Skin             string
	Player           int
	Hitpoints        int
	Mana             int
	RandomItemSetPtr int
	DroppedItemSets  []ItemSet
	Gold             int
	TargetAcquisition float32
	Hero             UnitHero
	Inventory        []UnitInventory
	Abilities        []UnitAbility
	Random           UnitRandom
	Color            int
	Waygate          int
	ID               int
}

// UnitFlags mirrors WC3 unit instance flags.
type UnitFlags struct {
	Visible bool
}
