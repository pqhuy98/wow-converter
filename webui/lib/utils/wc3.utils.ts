import type { IconFrame } from '@/lib/models/icon-export.model';

/**
 * Generate Wc3 output path for an icon based on texture path and frame type
 * @param texturePath - WoW texture path (e.g., "interface/icons/inv_belt_18.blp")
 * @param frame - Icon frame type (e.g., "btn", "pas", "none")
 * @returns Wc3 output path (e.g., "ReplaceableTextures/CommandButtons/BTNinv_belt_18.blp")
 */
export function getWc3Path(texturePath: string, frame: IconFrame | string): string {
  const filename = texturePath.split('/').pop() ?? texturePath;
  const baseName = filename.replace(/\.(blp|png|jpg|jpeg)$/i, '');

  switch (frame) {
    case 'btn':
      return `ReplaceableTextures/CommandButtons/BTN${baseName}.blp`;
    case 'disbtn':
      return `ReplaceableTextures/CommandButtonsDisabled/DISBTN${baseName}.blp`;
    case 'pas':
      return `ReplaceableTextures/PassiveButtons/PAS${baseName}.blp`;
    case 'dispas':
      return `ReplaceableTextures/CommandButtonsDisabled/DISPAS${baseName}.blp`;
    case 'atc':
      return `ReplaceableTextures/CommandButtons/ATC${baseName}.blp`;
    case 'disatc':
      return `ReplaceableTextures/CommandButtonsDisabled/DISATC${baseName}.blp`;
    case 'upg':
      return `ReplaceableTextures/CommandButtons/UPG${baseName}.blp`;
    case 'att':
      return `ReplaceableTextures/CommandButtons/ATT${baseName}.blp`;
    case 'ssh':
      return `SSH${baseName}.blp`;
    case 'ssp':
      return `SSP${baseName}.blp`;
    case 'none':
      return filename;
    default:
      return filename;
  }
}
