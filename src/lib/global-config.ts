import { wowDataClient } from './wow-data-client/wow-data-client';

export interface Config {
  exportAssetDir: string
  assetPrefix: string
  rawModelScaleUp: number
  overrideModels: boolean
  overrideTextures: boolean
  mdx?: boolean
  infiniteExtentBoundRadiusThreshold: number
  isBulkExport?: boolean
  maxTextureSize?: number
}

export async function getDefaultConfig(): Promise<Config> {
  return {
    assetPrefix: 'wow',
    mdx: true,
    infiniteExtentBoundRadiusThreshold: 2000, // WC3 distance unit
    rawModelScaleUp: 56,
    overrideModels: true,
    overrideTextures: false,
    exportAssetDir: await wowDataClient.getAssetDir(),
  };
}
