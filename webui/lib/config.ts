import { Config } from "./models/export-character.model"

export const serverConfig = {
  wowExportAssetDir: "/Users/huy/Documents/wow-export/assets",
  isSharedHosting: false,
}

export const setServerConfig = (config: Config) => {
  Object.assign(serverConfig, config)
}