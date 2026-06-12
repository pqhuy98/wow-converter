/** Normalize a local WoW install path for display and filesystem use. */
export function normalizeInstallDirectory(path: string): string {
  const trimmed = path.trim();
  if (!trimmed) return '';

  const drivePath = trimmed.match(/^([a-zA-Z]:)(.*)$/);
  if (drivePath) {
    const body = drivePath[2].replace(/\//g, '\\').replace(/\\+/g, '\\');
    return drivePath[1] + body;
  }

  if (trimmed.startsWith('\\\\') || trimmed.startsWith('//')) {
    return trimmed.replace(/\//g, '\\');
  }

  return trimmed.replace(/\\+/g, '\\');
}
