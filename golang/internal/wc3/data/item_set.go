package data

// DroppableItem is an item with drop chance.
type DroppableItem struct {
	ItemID string
	Chance int32
}

// ItemSet is a doodad/unit random item set.
type ItemSet struct {
	Items []DroppableItem
}
