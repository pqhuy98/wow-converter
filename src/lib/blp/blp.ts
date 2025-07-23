/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-call */
/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import fs from 'fs';
import { createRequire } from 'module';
import path from 'path';

import { png2BlpJs } from './blp-js';

const require = createRequire(import.meta.url);

// eslint-disable-next-line @typescript-eslint/no-explicit-any
let Image: any;
let TYPE_BLP: number;

if (process.platform === 'win32') {
  ({
    Image, TYPE_BLP,
  } = require('./bin/blp-preview/win32-x64-binding.node'));
} else if (process.platform === 'darwin' && process.arch === 'x64') {
  ({
    Image, TYPE_BLP,
  } = require('./bin/blp-preview/darwin-arm64-binding.node'));
}

export async function pngToBlp(pngPath: string, blpPath: string) {
  console.log('pngToBlp', pngPath, blpPath);
  if (!Image) {
    console.log('Using custom png2BlpJs');
    await png2BlpJs(pngPath, blpPath);
    return;
  }

  const img = new Image();
  const buf = fs.readFileSync(pngPath);
  img.loadFromBuffer(buf, 0, buf.length);
  fs.mkdirSync(path.dirname(blpPath), { recursive: true });
  fs.writeFileSync(blpPath, img.toBuffer(TYPE_BLP));
}
