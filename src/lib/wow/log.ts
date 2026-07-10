/**
 * Minimal logging shim replacing wow.export's log module (src/js/log.js).
 * Uses util.format semantics (%s/%d) like the original.
 */
import util from 'util';

const enabled = process.env.WOW_DATA_LOG !== '0';
const prefix = process.env.WOW_LOG_PREFIX ?? 'ts';

let loadingDepth = 0;
let latestLoadingMessageText = '';

function formatMessage(args: unknown[]): string {
  return args.length > 0 && typeof args[0] === 'string'
    ? util.format(...(args as [string, ...unknown[]]))
    : args.map((a) => util.inspect(a)).join(' ');
}

export function beginLoadingProgress(): void {
  loadingDepth += 1;
  if (loadingDepth === 1) latestLoadingMessageText = '';
}

export function endLoadingProgress(): void {
  if (loadingDepth > 0) loadingDepth -= 1;
}

export function isLoadingProgressActive(): boolean {
  return loadingDepth > 0;
}

export function latestLoadingMessage(): string {
  return latestLoadingMessageText;
}

export function write(...args: unknown[]): void {
  const line = formatMessage(args);
  if (loadingDepth > 0) latestLoadingMessageText = line;
  if (!enabled) return;
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

export default {
  write, timeLog, timeEnd, beginLoadingProgress, endLoadingProgress, isLoadingProgressActive, latestLoadingMessage,
};
