package character

import animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"

// Inventory types from WoW item metadata.
const (
	InventoryWeapon         = 13
	InventoryShield         = 14
	InventoryRanged         = 15
	InventoryTwoHanded      = 17
	InventoryWeaponMainHand = 21
	InventoryWeaponOffHand  = 22
	InventoryHoldable       = 23
	InventoryThrown         = 25
	InventoryRangedRight    = 26
	InventoryRelic          = 28
)

// InventoryTypeToEquipmentSlot maps item inventory type to WoW attachment ID.
func InventoryTypeToEquipmentSlot(inventoryType, idx int) (animmap.WoWAttachmentID, bool) {
	switch inventoryType {
	case InventoryWeapon, InventoryRanged, InventoryRangedRight, InventoryTwoHanded,
		InventoryWeaponMainHand, InventoryWeaponOffHand, InventoryHoldable, InventoryThrown, InventoryRelic:
		if idx == 0 {
			return animmap.WoWAttachmentHandRight, true
		}
		return animmap.WoWAttachmentHandLeft, true
	case InventoryShield:
		return animmap.WoWAttachmentShield, true
	default:
		return 0, false
	}
}

// GuessAttackTag infers attack animation tag from equipped items.
func GuessAttackTag(inventoryTypeR, inventoryTypeL int) animmap.AttackTag {
	if inventoryTypeR == InventoryRangedRight {
		return animmap.AttackTagRifle
	}
	if inventoryTypeR == InventoryRanged || inventoryTypeL == InventoryRanged {
		return animmap.AttackTagBow
	}
	if inventoryTypeR == InventoryThrown {
		return animmap.AttackTagThrown
	}
	if inventoryTypeR == InventoryTwoHanded && inventoryTypeL == 0 {
		return animmap.AttackTag2H
	}
	rightCanMelee := inventoryTypeR == InventoryWeapon ||
		inventoryTypeR == InventoryWeaponMainHand ||
		inventoryTypeR == InventoryWeaponOffHand ||
		inventoryTypeR == InventoryTwoHanded
	leftCanMelee := inventoryTypeL == InventoryWeapon ||
		inventoryTypeL == InventoryWeaponMainHand ||
		inventoryTypeL == InventoryWeaponOffHand ||
		inventoryTypeL == InventoryTwoHanded
	if rightCanMelee || leftCanMelee {
		return animmap.AttackTag1H
	}
	return animmap.AttackTagUnarmed
}
