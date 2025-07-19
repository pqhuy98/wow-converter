/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-call */
/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import fs from 'fs';
import { createRequire } from 'module';
import path from 'path';

const require = createRequire(import.meta.url);

// eslint-disable-next-line @typescript-eslint/naming-convention, @typescript-eslint/no-unsafe-assignment
const {
  Image, TYPE_PNG, TYPE_JPEG, TYPE_BLP,
// eslint-disable-next-line @typescript-eslint/no-unsafe-call, no-eval, @typescript-eslint/no-var-requires
} = require('./bin/blp-preview/win32-x64-binding.node');

export function blp2Image(blpPath: string, distPath: string, type: 'png' | 'jpg' | 'blp' = 'png') {
  const img = new Image();
  const buf = fs.readFileSync(blpPath);
  img.loadFromBuffer(buf, 0, buf.length);
  fs.mkdirSync(path.dirname(distPath), { recursive: true });
  if (type === 'png') {
    fs.writeFileSync(distPath, img.toBuffer(TYPE_PNG));
  } else if (type === 'blp') {
    fs.writeFileSync(distPath, img.toBuffer(TYPE_BLP));
  } else {
    fs.writeFileSync(distPath, img.toBuffer(TYPE_JPEG));
  }
}
