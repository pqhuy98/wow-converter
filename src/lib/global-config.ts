import { wowExportClient } from './wowexport-client/wowexport-client';

export interface Config {
  wowExportAssetDir: string
  assetPrefix: string
  rawModelScaleUp: number
  overrideModels: boolean
  mdx?: boolean
  infiniteExtentBoundRadiusThreshold: number
  isBulkExport?: boolean
}

export async function getDefaultConfig(): Promise<Config> {
  return {
    assetPrefix: 'wow',
    mdx: true,
    infiniteExtentBoundRadiusThreshold: 2000, // WC3 distance unit
    rawModelScaleUp: 56,
    overrideModels: true,
    wowExportAssetDir: await wowExportClient.getAssetDir(),
  };
}
