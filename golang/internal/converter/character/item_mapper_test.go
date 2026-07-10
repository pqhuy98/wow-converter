package character

import (
	"testing"

	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
)

func TestInventoryTypeToEquipmentSlot(t *testing.T) {
	tests := []struct {
		inventoryType int
		idx           int
		want          animmap.WoWAttachmentID
		ok            bool
	}{
		{InventoryWeapon, 0, animmap.WoWAttachmentHandRight, true},
		{InventoryWeapon, 1, animmap.WoWAttachmentHandLeft, true},
		{InventoryWeaponMainHand, 0, animmap.WoWAttachmentHandRight, true},
		{InventoryWeaponOffHand, 1, animmap.WoWAttachmentHandLeft, true},
		{InventoryShield, 0, animmap.WoWAttachmentShield, true},
		{InventoryShield, 1, animmap.WoWAttachmentShield, true},
		{99, 0, 0, false},
	}

	for _, tc := range tests {
		got, ok := InventoryTypeToEquipmentSlot(tc.inventoryType, tc.idx)
		if ok != tc.ok || got != tc.want {
			t.Errorf("InventoryTypeToEquipmentSlot(%d, %d) = (%d, %v), want (%d, %v)",
				tc.inventoryType, tc.idx, got, ok, tc.want, tc.ok)
		}
	}
}
