/**
 * Configuration for the native WoW reader, mirroring the wow.export config keys
 * that the headless read/export pipeline depends on (src/default_config.jsonc).
 *
 * Export-shaping values are fixed to the configuration wow-converter always
 * pushed to wow.export via syncConfig() (see wowexport-client.ts desiredConfig),
 * so native exports match the REST pipeline's artifacts exactly.
 */
import { EXPORT_PATH } from '@/lib/wow/formats/constants';

export interface WowReaderConfig {
  // Remote data sources
  listfileURL: string;
  listfileFallbackURL: string;
  listfileCacheRefresh: number; // days
  dbdURL: string;
  dbdFallbackURL: string;
  tactKeysURL: string;
  tactKeysFallbackURL: string;
  cacheExpiry: number; // days

  // CASC
  cascLocale: number;
  enableUnknownFiles: boolean;

  // Export shaping (matches wowexport-client desiredConfig + defaults)
  copyMode: string;
  listfileSortByID: boolean;
  listfileShowFileDataIDs: boolean;
  enableM2Skins: boolean;
  enableSharedTextures: boolean;
  enableSharedChildren: boolean;
  enableAbsoluteMTLPaths: boolean;
  enableAbsoluteCSVPaths: boolean;
  removePathSpaces: boolean;
  removePathSpacesCopy: boolean;
  exportTextureFormat: string;
  exportModelFormat: string;
  exportChannelMask: number;
  exportM2Bones: boolean;
  exportM2Meta: boolean;
  exportWMOMeta: boolean;
  exportBLPMeta: boolean;
  exportFoliageMeta: boolean;
  exportNamedFiles: boolean;
  overwriteFiles: boolean;
  modelsExportSkin: boolean;
  modelsExportSkel: boolean;
  modelsExportBone: boolean;
  modelsExportAnim: boolean;
  modelsExportWMOGroups: boolean;
  modelsExportUV2: boolean;
  modelsExportTextures: boolean;
  modelsExportAlpha: boolean;
  modelsExportAnimations: boolean;
  modelsExportCollision: boolean;
  modelsExportWithBonePrefix: boolean;
  modelsExportPngIncrements: boolean;

  // Maps / ADT
  mapsIncludeWMO: boolean;
  mapsIncludeM2: boolean;
  mapsIncludeWMOSets: boolean;
  mapsIncludeFoliage: boolean;
  mapsIncludeLiquid: boolean;
  mapsIncludeGameObjects: boolean;
  mapsIncludeHoles: boolean;
  mapsExportRaw: boolean;
  exportMapQuality: number;
  splitLargeTerrainBakes: boolean;
  splitAlphaMaps: boolean;

  // Character
  chrIncludeBaseClothing: boolean;

  pathFormat: string;

  // Where exported artifacts are written on disk.
  exportDirectory: string;
}

/**
 * Mutable singleton config, analogous to wow.export's core.view.config.
 * Values mirror default_config.jsonc overridden by wow-converter's desiredConfig.
 */
export const wowConfig: WowReaderConfig = {
  listfileURL: 'https://github.com/wowdev/wow-listfile/releases/latest/download/community-listfile.csv',
  listfileFallbackURL: 'https://www.kruithne.net/wow.export/data/listfile/master',
  listfileCacheRefresh: 3,
  dbdURL: 'https://raw.githubusercontent.com/wowdev/WoWDBDefs/refs/heads/master/definitions/%s.dbd',
  dbdFallbackURL: 'https://www.kruithne.net/wow.export/data/dbd/?def=%s',
  tactKeysURL: 'https://raw.githubusercontent.com/wowdev/TACTKeys/master/WoW.txt',
  tactKeysFallbackURL: 'https://www.kruithne.net/wow.export/data/tact/wow',
  cacheExpiry: 7,

  cascLocale: 2, // enUS
  enableUnknownFiles: true,

  copyMode: 'FULL',
  listfileSortByID: false,
  listfileShowFileDataIDs: true,
  enableM2Skins: true,
  enableSharedTextures: true,
  enableSharedChildren: true,
  enableAbsoluteMTLPaths: false,
  enableAbsoluteCSVPaths: false,
  removePathSpaces: true,
  removePathSpacesCopy: true,
  exportTextureFormat: 'PNG',
  exportModelFormat: 'OBJ',
  exportChannelMask: 15,
  exportM2Bones: true,
  exportM2Meta: true,
  exportWMOMeta: true,
  exportBLPMeta: false,
  exportFoliageMeta: false,
  exportNamedFiles: true,
  overwriteFiles: true,
  modelsExportSkin: true,
  modelsExportSkel: true,
  modelsExportBone: true,
  modelsExportAnim: true,
  modelsExportWMOGroups: true,
  modelsExportUV2: true,
  modelsExportTextures: true,
  modelsExportAlpha: true,
  modelsExportAnimations: true,
  modelsExportCollision: true,
  modelsExportWithBonePrefix: true,
  modelsExportPngIncrements: true,

  mapsIncludeWMO: true,
  mapsIncludeM2: true,
  mapsIncludeWMOSets: true,
  mapsIncludeFoliage: true,
  mapsIncludeLiquid: false,
  mapsIncludeGameObjects: true,
  mapsIncludeHoles: true,
  mapsExportRaw: false,
  exportMapQuality: 4096,
  splitLargeTerrainBakes: true,
  splitAlphaMaps: true,

  chrIncludeBaseClothing: true,

  pathFormat: 'win32',

  exportDirectory: EXPORT_PATH,
};
