/**
 * JSON writer, ported from wow.export (src/js/3D/writers/JSONWriter.js).
 */
import { outputFileExists, writeOutputFile } from './output-sink';

export class JSONWriter {
  out: string;

  data: Record<string, unknown> = {};

  constructor(out: string) {
    this.out = out;
  }

  /** Add a property to this JSON. */
  addProperty(name: string, data: unknown): void {
    this.data[name] = data;
  }

  /** Produce the JSON file content. */
  getContent(minify = false): string {
    // Try the fastest stringify path first (no replacer). If the payload contains
    // BigInt values it will throw - fall back to a replacer only in that case.
    let jsonStr: string;
    try {
      jsonStr = JSON.stringify(this.data, null, minify ? undefined : '\t');
    } catch (err) {
      if (err instanceof Error && /BigInt/.test(err.message)) {
        jsonStr = JSON.stringify(
          this.data,
          (_key, value) => (typeof value === 'bigint' ? value.toString() : value),
          minify ? undefined : '\t',
        );
      } else {
        throw err;
      }
    }

    return `${jsonStr}\n`;
  }

  /** Write the JSON. */
  async write(overwrite = true, minify = false): Promise<void> {
    // If overwriting is disabled, check file existence.
    if (!overwrite && await outputFileExists(this.out)) return;

    await writeOutputFile(this.out, this.getContent(minify));
  }
}

export default JSONWriter;
