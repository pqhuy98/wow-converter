/** Extra clear hooks registered by wow-data-server (e.g. REST response memoization). */
const wowDataServerClearHooks = new Set<() => void>();

export function registerWowDataServerClearHook(fn: () => void): () => void {
  wowDataServerClearHooks.add(fn);
  return () => wowDataServerClearHooks.delete(fn);
}

export function runWowDataServerClearHooks(): void {
  for (const fn of wowDataServerClearHooks) fn();
}
