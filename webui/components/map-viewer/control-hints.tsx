'use client';

import { Mouse } from 'lucide-react';

import { TooltipHelp } from '@/components/common/tooltip-help';

export function MapViewerControlHints() {
  return (
    <TooltipHelp
      trigger={(
        <span className="inline-flex">
          <Mouse className="h-6 w-6 text-foreground drop-shadow" />
        </span>
      )}
      tooltips={(
        <div className="text-sm leading-5">
          <div><span className="font-semibold">Left drag</span>: select tiles</div>
          <div><span className="font-semibold">Shift + left drag</span>: deselect tiles</div>
          <div><span className="font-semibold">Ctrl + left drag</span>: paint select / deselect tiles</div>
          <div><span className="font-semibold">Right drag</span>: pan map</div>
          <div><span className="font-semibold">Esc</span>: clear all tiles</div>
        </div>
      )}
      asChild
    />
  );
}
