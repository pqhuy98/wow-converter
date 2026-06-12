/**
 * Parser for CDN config files (BuildConfig / CDNConfig),
 * ported from wow.export (src/js/casc/cdn-config.js).
 */
const KEY_VAR_PATTERN = /([^\s]+)\s?=\s?(.*)/;

/**
 * Convert a config key such as 'encoding-sizes' to 'encodingSizes'.
 */
function normalizeKey(key: string): string {
  const keyParts = key.split('-');

  if (keyParts.length === 1) return key;

  for (let i = 1, n = keyParts.length; i < n; i++) {
    const part = keyParts[i];
    keyParts[i] = part.charAt(0).toUpperCase() + part.slice(1);
  }

  return keyParts.join('');
}

export type CDNConfigEntries = Record<string, string>;

export function parseCDNConfig(data: string): CDNConfigEntries {
  const entries: CDNConfigEntries = {};
  const lines = data.split(/\r?\n/);

  // Valid config files start with "# " (e.g., "# Build Configuration", "# CDN Configuration")
  const hasValidHeader = lines.length > 0 && lines[0].trim().startsWith('# ');

  if (!hasValidHeader) throw new Error('Invalid CDN config: unexpected start of config');

  for (const line of lines) {
    // Skip empty lines/comments.
    if (line.trim().length === 0 || line.startsWith('#')) continue;

    const match = line.match(KEY_VAR_PATTERN);
    if (match === null) throw new Error('Invalid token encountered parsing CDN config');

    entries[normalizeKey(match[1])] = match[2];
  }

  return entries;
}

export default parseCDNConfig;
