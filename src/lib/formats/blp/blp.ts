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

export async function pngToBlp(png: string | Buffer, blpPath: string) {
  if (!Image) {
    console.log('Using custom png2BlpJs');
    await png2BlpJs(png, blpPath);
    return;
  }

  const img = new Image();
  const buf = typeof png === 'string' ? fs.readFileSync(png) : png;
  img.loadFromBuffer(buf, 0, buf.length);
  fs.mkdirSync(path.dirname(blpPath), { recursive: true });
  fs.writeFileSync(blpPath, img.toBuffer(TYPE_BLP));
}

export function readBlpSizeSync(blpPath: string): { width: number, height: number } | null {
  try {
    const fd = fs.openSync(blpPath, 'r');
    try {
      const header = Buffer.alloc(20);
      const bytesRead = fs.readSync(fd, header, 0, 20, 0);
      if (bytesRead < 20) {
        return null;
      }
      const magic = header.toString('ascii', 0, 4);
      if (magic !== 'BLP1' && magic !== 'BLP2') {
        return null;
      }
      const width = header.readUInt32LE(12);
      const height = header.readUInt32LE(16);
      return { width, height };
    } finally {
      fs.closeSync(fd);
    }
  } catch {
    return null;
  }
}
