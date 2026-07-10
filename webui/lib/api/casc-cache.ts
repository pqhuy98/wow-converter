/** Append active CASC buildKey so browser cache keys rotate on version switch. */
export function withCascBuild(url: string, buildKey?: string): string {
  if (!buildKey) return url;
  const join = url.includes('?') ? '&' : '?';
  return `${url}${join}build=${encodeURIComponent(buildKey)}`;
}
