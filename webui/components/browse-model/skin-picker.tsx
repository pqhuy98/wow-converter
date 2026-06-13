'use client';

import { memo } from 'react';

export type ModelSkinOption = {
  id: string;
  label: string;
  localRef: string;
};

export const SKIN_ROW_HEIGHT = 24;
const SKIN_HEADER_HEIGHT = 20;
const SKIN_PANEL_PADDING = 4;

export function skinPanelHeight(skinCount: number): number {
  if (skinCount <= 1) return SKIN_HEADER_HEIGHT + SKIN_PANEL_PADDING + SKIN_ROW_HEIGHT;
  return SKIN_HEADER_HEIGHT + (skinCount * SKIN_ROW_HEIGHT) + SKIN_PANEL_PADDING;
}

interface SkinPickerProps {
  skins: ModelSkinOption[];
  selectedSkinId: string | null;
  isBusy: boolean;
  loading?: boolean;
  error?: string | null;
  onSelect: (skin: ModelSkinOption) => void;
}

export const SkinPicker = memo(({
  skins,
  selectedSkinId,
  isBusy,
  loading = false,
  error = null,
  onSelect,
}: SkinPickerProps) => {
  const showList = skins.length > 1;

  return (
    <div className="border-t border-border/60 bg-muted/30 px-2 py-1">
      <div className="text-[10px] uppercase tracking-wide text-muted-foreground mb-0.5 pl-14">
        Skins
      </div>
      {loading ? (
        <div className="pl-14 pr-2 text-xs text-muted-foreground py-1">Loading skins…</div>
      ) : null}
      {error ? (
        <div className="pl-14 pr-2 text-xs text-destructive py-1">{error}</div>
      ) : null}
      {!loading && !error && !showList ? (
        <div className="pl-14 pr-2 text-xs text-muted-foreground py-1">
          {skins.length === 1 ? skins[0]!.label : 'No variants'}
        </div>
      ) : null}
      {!loading && !error && showList
        ? skins.map((skin) => {
          const isActive = selectedSkinId === skin.id;
          return (
            <button
              key={skin.id}
              type="button"
              disabled={isBusy}
              className={`w-full text-left pl-14 pr-2 flex items-center gap-2 text-xs rounded-sm ${
                isActive ? 'bg-primary/25 text-foreground' : 'hover:bg-accent text-foreground/80'
              } ${isBusy ? 'opacity-60 cursor-not-allowed' : 'cursor-pointer'}`}
              style={{ minHeight: SKIN_ROW_HEIGHT }}
              title={skin.localRef}
              onClick={(e) => {
                e.stopPropagation();
                if (!isBusy && !isActive) onSelect(skin);
              }}
            >
              <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${isActive ? 'bg-primary' : 'bg-muted-foreground/40'}`} />
              <span className="truncate">{skin.label}</span>
            </button>
          );
        })
        : null}
    </div>
  );
});

SkinPicker.displayName = 'SkinPicker';
