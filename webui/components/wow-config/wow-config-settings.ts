const INSTALL_DIR_KEY = 'wow-config-local-install-dir';
const PRODUCT_KEY = 'wow-config-product';

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

export function readStoredInstallDirectory(): string {
  if (typeof window === 'undefined') return '';
  try {
    const value = localStorage.getItem(INSTALL_DIR_KEY);
    return typeof value === 'string' ? normalizeInstallDirectory(value) : '';
  } catch {
    return '';
  }
}

export function writeStoredInstallDirectory(path: string): void {
  if (typeof window === 'undefined') return;
  try {
    const normalized = normalizeInstallDirectory(path);
    if (normalized) {
      localStorage.setItem(INSTALL_DIR_KEY, normalized);
    } else {
      localStorage.removeItem(INSTALL_DIR_KEY);
    }
  } catch {
    // ignore quota / private mode errors
  }
}

export function readStoredProduct(): string | null {
  if (typeof window === 'undefined') return null;
  try {
    const value = localStorage.getItem(PRODUCT_KEY);
    return typeof value === 'string' && value.length > 0 ? value : null;
  } catch {
    return null;
  }
}

export function writeStoredProduct(product: string | null): void {
  if (typeof window === 'undefined') return;
  try {
    if (product) {
      localStorage.setItem(PRODUCT_KEY, product);
    } else {
      localStorage.removeItem(PRODUCT_KEY);
    }
  } catch {
    // ignore quota / private mode errors
  }
}
