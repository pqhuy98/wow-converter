import type { WowReaderConfig } from './config';

/** Keys the converter may sync via /rest/setConfig (mirrors wow-data-client desiredConfig). */
export const SETTABLE_CONFIG_KEYS = new Set<keyof WowReaderConfig>([
  'copyMode',
  'listfileShowFileDataIDs',
  'enableM2Skins',
  'enableSharedTextures',
  'enableSharedChildren',
  'enableAbsoluteMTLPaths',
  'enableAbsoluteCSVPaths',
  'removePathSpaces',
  'removePathSpacesCopy',
  'exportTextureFormat',
  'exportModelFormat',
  'exportM2Bones',
  'exportM2Meta',
  'exportWMOMeta',
  'modelsExportSkin',
  'modelsExportSkel',
  'modelsExportBone',
  'modelsExportAnim',
  'modelsExportUV2',
  'modelsExportTextures',
  'modelsExportAlpha',
  'modelsExportAnimations',
  'modelsExportCollision',
]);

export function isSettableConfigKey(key: string): key is keyof WowReaderConfig {
  return SETTABLE_CONFIG_KEYS.has(key as keyof WowReaderConfig);
}
