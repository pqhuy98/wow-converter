import { AttackTag } from '@/lib/objmdl/animation/animation_mapper';
import { WoWAttachmentID } from '@/lib/objmdl/animation/bones_mapper';

export enum InventoryType {
  WEAPON = 13,
  SHIELD = 14,
  RANGED = 15,
  RANGEDRIGHT = 26,
  TWO_HANDED_WEAPON = 17,
  WEAPONMAINHAND = 21,
  WEAPONOFFHAND = 22,
  HOLDABLE = 23,
  THROWN = 25,
  RELIC = 28,
}

export function inventoryTypeToEquipmentSlot(inventoryType: number, idx: number): WoWAttachmentID | undefined {
  switch (inventoryType) {
    case InventoryType.WEAPON:
    case InventoryType.RANGED:
    case InventoryType.RANGEDRIGHT:
    case InventoryType.TWO_HANDED_WEAPON:
    case InventoryType.WEAPONMAINHAND:
    case InventoryType.WEAPONOFFHAND:
    case InventoryType.HOLDABLE:
    case InventoryType.THROWN:
    case InventoryType.RELIC:
      return idx === 0 ? WoWAttachmentID.HandRight : WoWAttachmentID.HandLeft;
    case InventoryType.SHIELD:
      return WoWAttachmentID.Shield;
    default:
      return undefined;
  }
}

export function guessAttackTag(inventoryTypeR: number, inventoryTypeL: number): AttackTag {
  console.log('inventoryTypeR', inventoryTypeR);
  console.log('inventoryTypeL', inventoryTypeL);
  if (inventoryTypeR === InventoryType.RANGEDRIGHT) {
    return 'Rifle';
  }
  if (inventoryTypeL === InventoryType.RANGED || inventoryTypeR === InventoryType.RANGEDRIGHT) {
    return 'Bow';
  }

  if (inventoryTypeR === InventoryType.THROWN) {
    return 'Thrown';
  }

  if (inventoryTypeR === InventoryType.TWO_HANDED_WEAPON && !inventoryTypeL) {
    return '2H';
  }

  const rightCanMelee = inventoryTypeR === InventoryType.WEAPON
    || inventoryTypeR === InventoryType.WEAPONMAINHAND
    || inventoryTypeR === InventoryType.WEAPONOFFHAND
    || inventoryTypeR === InventoryType.TWO_HANDED_WEAPON;
  const leftCanMelee = inventoryTypeL === InventoryType.WEAPON
    || inventoryTypeL === InventoryType.WEAPONMAINHAND
    || inventoryTypeL === InventoryType.WEAPONOFFHAND
    || inventoryTypeL === InventoryType.TWO_HANDED_WEAPON;

  if (rightCanMelee || leftCanMelee) {
    return '1H';
  }

  return 'Unarmed';
}
