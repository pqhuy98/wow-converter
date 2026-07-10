/**
 * Minimal logging shim replacing wow.export's log module (src/js/log.js).
 * Uses util.format semantics (%s/%d) like the original.
 */
import util from 'util';

const enabled = process.env.WOW_DATA_LOG !== '0';
const prefix = process.env.WOW_LOG_PREFIX ?? 'ts';

export function write(...args: unknown[]): void {
  if (!enabled) return;
  const line = args.length > 0 && typeof args[0] === 'string'
    ? util.format(...(args as [string, ...unknown[]]))
    : args.map((a) => util.inspect(a)).join(' ');
  console.log(`[${prefix}][wow] ${line}`);
}

/** Stack of timestamps for timeLog/timeEnd pairs (mirrors wow.export timeLog). */
const timers: number[] = [];

export function timeLog(): void {
  timers.push(Date.now());
}

export function timeEnd(...args: unknown[]): void {
  const start = timers.pop();
  const elapsed = start === undefined ? -1 : Date.now() - start;
  if (args.length > 0 && typeof args[0] === 'string') write(`${args[0]} (took ${elapsed}ms)`, ...args.slice(1));
  else write(`Timer ended (took ${elapsed}ms)`);
}

export default { write, timeLog, timeEnd };
