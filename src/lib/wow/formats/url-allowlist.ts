const DISALLOWED_FETCH_HOSTS = new Set(['localhost']);

function isPrivateIP(host: string): boolean {
  const m = host.match(/^(\d+)\.(\d+)\.(\d+)\.(\d+)$/);
  if (!m) return false;
  const a = Number(m[1]);
  const b = Number(m[2]);
  if (a === 10) return true;
  if (a === 127) return true;
  if (a === 169 && b === 254) return true;
  if (a === 192 && b === 168) return true;
  if (a === 172 && b >= 16 && b <= 31) return true;
  return false;
}

function isBlizzardCdnHost(hostname: string): boolean {
  const h = hostname.toLowerCase();
  return /\.blizzard\.com$/i.test(h) || /\.battle\.net$/i.test(h);
}

function isAllowedFetchHost(hostname: string): boolean {
  const h = hostname.toLowerCase();
  if (isBlizzardCdnHost(h)) return true;
  if (h === 'github.com' || h.endsWith('.github.com')) return true;
  if (h.endsWith('.githubusercontent.com')) return true;
  if (h === 'kruithne.net' || h.endsWith('.kruithne.net')) return true;
  return false;
}

/** Ensures url is HTTPS to an allowlisted public host (blocks SSRF to metadata/private nets). */
export function validateFetchUrl(raw: string): URL {
  let parsed: URL;
  try {
    parsed = new URL(raw);
  } catch {
    throw new Error('Invalid fetch URL');
  }
  if (parsed.protocol !== 'https:') {
    throw new Error('Fetch URL must use HTTPS');
  }
  if (parsed.username || parsed.password) {
    throw new Error('Fetch URL must not include credentials');
  }
  const host = parsed.hostname.toLowerCase();
  if (!host || DISALLOWED_FETCH_HOSTS.has(host) || isPrivateIP(host) || !isAllowedFetchHost(host)) {
    throw new Error(`Fetch URL host not allowed: ${host}`);
  }
  return parsed;
}
