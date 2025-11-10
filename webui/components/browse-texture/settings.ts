import type { IconResizeMode, IconSize, IconStyle } from '@/lib/models/icon-export.model';

import type { SelectionItem } from './icon-exporter';

const STORAGE_KEY = 'icon-exporter-settings';

const DEFAULT_STYLE: IconStyle = 'classic-hd-2.0';
const DEFAULT_SIZE: IconSize = '128x128';
const DEFAULT_RESIZE_MODE: IconResizeMode = 'normal';

interface IconExporterSettings {
  style: IconStyle;
  size: IconSize;
  resizeMode?: IconResizeMode;
  selection: SelectionItem[];
}

const DEFAULT_SETTINGS: IconExporterSettings = {
  style: DEFAULT_STYLE,
  size: DEFAULT_SIZE,
  resizeMode: undefined,
  selection: [],
};

function loadSettings(): IconExporterSettings {
  if (typeof window === 'undefined') return DEFAULT_SETTINGS;
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return DEFAULT_SETTINGS;
    const parsed = JSON.parse(stored) as Partial<IconExporterSettings>;

    // Validate and merge with defaults
    const settings: IconExporterSettings = {
      style: DEFAULT_STYLE,
      size: DEFAULT_SIZE,
      resizeMode: undefined,
      selection: [],
    };

    // Validate style
    if (parsed.style && ['classic-sd', 'reforged-hd', 'classic-hd-2.0'].includes(parsed.style)) {
      settings.style = parsed.style;
    }

    // Validate size
    if (parsed.size && ['64x64', '128x128', '256x256'].includes(parsed.size)) {
      settings.size = parsed.size;
    }

    // Validate resizeMode
    if (parsed.resizeMode && ['normal', 'ai'].includes(parsed.resizeMode)) {
      settings.resizeMode = parsed.resizeMode;
    }

    // Validate selection
    if (Array.isArray(parsed.selection)) {
      settings.selection = parsed.selection;
    }

    return settings;
  } catch {
    return DEFAULT_SETTINGS;
  }
}

function saveSettings(settings: Partial<IconExporterSettings>): void {
  if (typeof window === 'undefined') return;
  try {
    const current = loadSettings();
    const updated: IconExporterSettings = {
      ...current,
      ...settings,
    };
    localStorage.setItem(STORAGE_KEY, JSON.stringify(updated));
  } catch {
    // Ignore storage errors
  }
}

export function loadStyleFromStorage(): IconStyle {
  return loadSettings().style;
}

export function saveStyleToStorage(style: IconStyle): void {
  saveSettings({ style });
}

export function loadSizeFromStorage(): IconSize {
  return loadSettings().size;
}

export function saveSizeToStorage(size: IconSize): void {
  saveSettings({ size });
}

export function loadResizeModeFromStorage(): IconResizeMode | undefined {
  return loadSettings().resizeMode;
}

export function saveResizeModeToStorage(resizeMode: IconResizeMode | undefined): void {
  saveSettings({ resizeMode });
}

export { DEFAULT_SIZE, DEFAULT_RESIZE_MODE };

export function loadSelectionFromStorage(): SelectionItem[] {
  return loadSettings().selection;
}

export function saveSelectionToStorage(selection: SelectionItem[]): void {
  saveSettings({ selection });
}

export { DEFAULT_STYLE };
