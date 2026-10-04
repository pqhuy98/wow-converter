'use client';

import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';

const labels = ['Wow-converter', 'Wowhead'];

export function ReportImages({ images }: { images: string[] }) {
  const ordered = images.length > 1 ? [images[1], images[0]] : images;
  const [active, setActive] = useState<number | null>(null);
  useEffect(() => {
    if (active === null) return undefined;
    const key = (event: KeyboardEvent) => {
      if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
        event.preventDefault();
        setActive((current) => (current === 0 ? 1 : 0));
      }
    };
    window.addEventListener('keydown', key);
    return () => window.removeEventListener('keydown', key);
  }, [active]);

  return <div onClick={(event) => event.stopPropagation()}>
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
      {ordered.map((image, index) => <button key={image} type="button" className="text-left" onClick={(event) => { event.stopPropagation(); setActive(index); }}>
        <span className="block text-sm font-medium mb-1">{labels[index]}</span>
        <img src={image} alt={`${labels[index]} six-view comparison`} width={1920} height={800} className="w-full aspect-[12/5] object-contain bg-neutral-900 rounded border" />
        <span className="text-xs text-muted-foreground">Click to enlarge</span>
      </button>)}
    </div>
    <Dialog open={active !== null} onOpenChange={(open) => { if (!open) setActive(null); }}>
      <DialogContent className="max-w-[98vw] w-[98vw] h-[95vh] flex flex-col" onClick={(event) => event.stopPropagation()}>
        <DialogTitle>{active === null ? 'Comparison' : labels[active]}</DialogTitle>
        <div className="flex-1 min-h-0 flex items-center justify-center bg-neutral-900">
          {active !== null && <img src={ordered[active]} alt={`${labels[active]} six-view comparison enlarged`} width={1920} height={800} className="w-full h-full object-contain" />}
        </div>
        <div className="flex justify-center items-center gap-4">
          <Button variant="outline" onClick={() => setActive((current) => (current === 0 ? 1 : 0))} aria-label="Previous screenshot">←</Button>
          <span className="w-40 text-center text-sm">{active === null ? '' : labels[active]} · {(active ?? 0) + 1}/2</span>
          <Button variant="outline" onClick={() => setActive((current) => (current === 0 ? 1 : 0))} aria-label="Next screenshot">→</Button>
        </div>
      </DialogContent>
    </Dialog>
  </div>;
}
