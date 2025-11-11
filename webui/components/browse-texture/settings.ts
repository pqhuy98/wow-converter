import type { IconResizeMode, IconSize, IconStyle } from '@/lib/models/icon-export.model';

import type { SelectionItem } from './icon-exporter';

const STORAGE_KEY = 'icon-exporter-settings';

interface IconExporterSettings {
  style: IconStyle;
  size: IconSize;
  resizeMode?: IconResizeMode;
  selection: SelectionItem[];
}

const DEFAULT_SETTINGS: IconExporterSettings = {
  style: 'classic-hd-2.0',
  size: 'original',
  resizeMode: undefined,
  selection: [],
};

export function loadSettings(): IconExporterSettings {
  if (typeof window === 'undefined') return DEFAULT_SETTINGS;
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return DEFAULT_SETTINGS;
    const parsed = JSON.parse(stored) as Partial<IconExporterSettings>;

    // Validate and merge with defaults
    const settings: IconExporterSettings = {
      style: DEFAULT_SETTINGS.style,
      size: DEFAULT_SETTINGS.size,
      resizeMode: DEFAULT_SETTINGS.resizeMode,
      selection: [],
    };

    // Validate style
    if (parsed.style && ['classic-sd', 'reforged-hd', 'classic-hd-2.0'].includes(parsed.style)) {
      settings.style = parsed.style;
    }

    // Validate size
    if (parsed.size && ['64x64', '128x128', '256x256', 'original'].includes(parsed.size)) {
      settings.size = parsed.size;
    }

    // Validate resizeMode
    if (parsed.resizeMode && ['normal', 'ai'].includes(parsed.resizeMode)) {
      settings.resizeMode = parsed.resizeMode;
    }

    // Validate selection - if it doesn't match expected structure, use default empty array
    if (Array.isArray(parsed.selection)) {
      settings.selection = parsed.selection.filter((item): item is SelectionItem => (
        typeof item === 'object'
        && item !== null
        && typeof item.texturePath === 'string'
        && typeof item.style === 'string'
        && typeof item.groupIndex === 'number'
        && Array.isArray(item.variants)
        && typeof item.size === 'string'
        && typeof item.id === 'string'
        && typeof item.outputName === 'string'
      ));
    }

    return settings;
  } catch {
    return DEFAULT_SETTINGS;
  }
}

export function saveSettings(settings: Partial<IconExporterSettings>): void {
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
