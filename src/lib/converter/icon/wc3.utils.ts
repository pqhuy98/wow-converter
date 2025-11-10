import type { IconFrame } from './schemas';

/**
 * Generate Wc3 output path for an icon based on texture path and frame type
 * @param texturePath - WoW texture path (e.g., "interface/icons/inv_belt_18.blp")
 * @param frame - Icon frame type (e.g., "btn", "pas", "none")
 * @returns Wc3 output path (e.g., "ReplaceableTextures\\CommandButtons\\BTN_inv_belt_18.blp")
 */
export function getWc3Path(texturePath: string, frame: IconFrame | string): string {
  const filename = texturePath.split('/').pop() ?? texturePath;
  const baseName = filename.replace(/\.(blp|png|jpg|jpeg)$/i, '');

  switch (frame) {
    case 'btn':
      return `ReplaceableTextures\\CommandButtons\\BTN_${baseName}.blp`;
    case 'disbtn':
      return `ReplaceableTextures\\CommandButtonsDisabled\\DISBTN_${baseName}.blp`;
    case 'pas':
      return `ReplaceableTextures\\PassiveButtons\\PAS_${baseName}.blp`;
    case 'dispas':
      return `ReplaceableTextures\\CommandButtonsDisabled\\DISPAS_${baseName}.blp`;
    case 'atc':
      return `ReplaceableTextures\\CommandButtons\\ATC_${baseName}.blp`;
    case 'disatc':
      return `ReplaceableTextures\\CommandButtonsDisabled\\DISATC_${baseName}.blp`;
    case 'upg':
      return `ReplaceableTextures\\CommandButtons\\UPG_${baseName}.blp`;
    case 'att':
      return `ReplaceableTextures\\CommandButtons\\ATT_${baseName}.blp`;
    case 'ssh':
      return `scorescreen-hero-${baseName}.blp`;
    case 'ssp':
      return `scorescreen-player-${baseName}.blp`;
    case 'none':
      return filename;
    default:
      return filename;
  }
}
