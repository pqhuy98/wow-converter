import fsExtra from 'fs-extra';

export const isSharedHosting = process.env.IS_SHARED_HOSTING === 'true';

// A value that changes on every server start so version hashes differ between restarts.
export const serverDeployTime = Date.now().toString();

export const ceOutputPath = 'exported-assets';
fsExtra.ensureDirSync(ceOutputPath);
