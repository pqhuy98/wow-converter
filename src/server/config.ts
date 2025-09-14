import fsExtra from 'fs-extra';

export const isSharedHosting = process.env.IS_SHARED_HOSTING === 'true';

export const isDev = process.env.NODE_ENV === 'development';

// A value that changes on every server start so version hashes differ between restarts.
export const serverDeployTime = Date.now().toString();

export const outputDir = 'exported-assets';
fsExtra.ensureDirSync(outputDir);
export const outputDirBrowse = 'exported-assets-browse';
fsExtra.ensureDirSync(outputDirBrowse);
