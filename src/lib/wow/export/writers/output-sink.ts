/**
 * Export artifacts are written to disk under wowConfig.exportDirectory.
 */
import fs from 'fs/promises';
import path from 'path';

export interface OutputSink {
  writeFile(filePath: string, data: Buffer | string): Promise<void>;
  fileExists(filePath: string): Promise<boolean>;
}

const diskSink: OutputSink = {
  async writeFile(filePath: string, data: Buffer | string): Promise<void> {
    await fs.mkdir(path.dirname(filePath), { recursive: true });
    await fs.writeFile(filePath, data);
  },

  async fileExists(filePath: string): Promise<boolean> {
    try {
      await fs.access(filePath);
      return true;
    } catch {
      return false;
    }
  },
};

export function writeOutputFile(filePath: string, data: Buffer | string): Promise<void> {
  return diskSink.writeFile(filePath, data);
}

export function outputFileExists(filePath: string): Promise<boolean> {
  return diskSink.fileExists(filePath);
}
