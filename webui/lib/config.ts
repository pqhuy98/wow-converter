import { Config } from './models/export-character.model';

export const serverConfig = {
  wowExportAssetDir: '',
  isSharedHosting: false,
};

export const setServerConfig = (config: Config) => {
  Object.assign(serverConfig, config);
};
