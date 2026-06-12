/**
 * CSV writer, ported from wow.export (src/js/3D/writers/CSVWriter.js).
 * Builds content in memory and flushes through the output sink.
 */
import { outputFileExists, writeOutputFile } from './output-sink';

export class CSVWriter {
  out: string;

  fields: string[] = [];

  rows: Record<string, unknown>[] = [];

  constructor(out: string) {
    this.out = out;
  }

  /** Add fields to this CSV. */
  addField(...fields: string[]): void {
    this.fields.push(...fields);
  }

  /** Add a row to this CSV. */
  addRow(row: Record<string, unknown>): void {
    this.rows.push(row);
  }

  /** Escape a CSV field value if it contains special characters. */
  escapeCSVField(value: unknown): string {
    if (value === null || value === undefined) return '';

    const str = String(value);
    if (str.includes(';') || str.includes('"') || str.includes('\n')) return `"${str.replace(/"/g, '""')}"`;

    return str;
  }

  /** Get the CSV content as a string (header + rows, LF line endings). */
  getContent(): string {
    const lines: string[] = [];
    lines.push(this.fields.map((field) => this.escapeCSVField(field)).join(';'));

    const nFields = this.fields.length;
    for (const row of this.rows) {
      const rowOut = new Array<string>(nFields);
      for (let i = 0; i < nFields; i++) rowOut[i] = this.escapeCSVField(row[this.fields[i]]);

      lines.push(rowOut.join(';'));
    }

    return `${lines.join('\n')}\n`;
  }

  /** Write the CSV through the output sink. */
  async write(overwrite = true): Promise<void> {
    // Don't bother writing an empty CSV file.
    if (this.rows.length === 0) return;

    // If overwriting is disabled, check file existence.
    if (!overwrite && await outputFileExists(this.out)) return;

    await writeOutputFile(this.out, this.getContent());
  }
}

export default CSVWriter;
