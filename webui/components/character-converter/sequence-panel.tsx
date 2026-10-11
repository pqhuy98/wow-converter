'use client';

import { ListChecks, Pencil, RefreshCw, RotateCcw, Search, X } from 'lucide-react';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';

import { TooltipHelp } from '@/components/common/tooltip-help';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import {
  Character, ModelMetadata, ReportSequence, SequenceSelection,
} from '@/lib/models/export-character.model';

export function SequencePanel({
  character, buildKey, skinId, playbackSequences, busy, onPreview, onApply,
}: {
  character: Character
  buildKey: string
  skinId?: string
  playbackSequences: ReportSequence[]
  busy: boolean
  onPreview: (name: string) => void
  onApply: (selection: SequenceSelection | undefined) => Promise<boolean>
}) {
  const [metadata, setMetadata] = useState<ModelMetadata>();
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<number[]>([]);
  const [custom, setCustom] = useState(!!character.sequenceSelection);
  const [submitting, setSubmitting] = useState(false);
  const [playing, setPlaying] = useState<number>();
  const [notice, setNotice] = useState('');
  const listRef = useRef<HTMLDivElement>(null);
  const scrollPosition = useRef(0);

  useLayoutEffect(() => {
    if (listRef.current) listRef.current.scrollTop = scrollPosition.current;
  }, [playbackSequences]);

  useEffect(() => {
    const controller = new AbortController();
    void (async () => {
      try {
        const response = await fetch('/api/model/metadata', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ character, skinId }),
          signal: controller.signal,
        });
        const data = await response.json();
        if (!response.ok) throw new Error(data.error ?? 'Could not load model sequences');
        const meta: ModelMetadata = data;
        if (controller.signal.aborted) return;
        if (meta.buildKey !== buildKey) throw new Error('WoW build changed. Reopen sequence selection.');
        const selection = character.sequenceSelection;
        setMetadata(meta);
        if (selection && (selection.fileDataID !== meta.fileDataID || selection.buildKey !== meta.buildKey || selection.indices.some((index) => !meta.sequences.some((sequence) => sequence.index === index)))) {
          setSelected([]);
          throw new Error('The saved selection no longer matches this model. Use defaults to reset it.');
        }
        setSelected(selection?.indices ?? meta.sequences.filter((sequence) => sequence.defaultIncluded).map((sequence) => sequence.index));
      } catch (failure) {
        if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : 'Could not load sequences');
      }
    })();
    return () => controller.abort();
  }, [character, buildKey, skinId]);

  const submit = async () => {
    if (!metadata) return;
    setSubmitting(true);
    setError('');
    try {
      const selection = custom ? { fileDataID: metadata.fileDataID, buildKey: metadata.buildKey, indices: [...selected].sort((a, b) => a - b) } : undefined;
      if (!await onApply(selection)) setError('Export was not accepted. Your selection is preserved; correct the export settings and try again.');
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : 'Export could not be submitted');
    } finally { setSubmitting(false); }
  };

  const sequences = metadata?.sequences.filter((sequence) => `${sequence.wc3Name} ${sequence.wowName} ${sequence.wowVariant}`.toLowerCase().includes(query.toLowerCase())) ?? [];
  return (
    <section aria-label="Edit animations" className="flex flex-col flex-1 min-h-0 min-w-0 w-full p-2 gap-2 text-[hsl(var(--viewer-sidebar-fg))]">
      {!metadata && !error && <p role="status">Loading source sequences…</p>}
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      {metadata && <>
        <div className="relative shrink-0">
          <Search aria-hidden="true" className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[hsl(var(--viewer-muted))] pointer-events-none" />
          <Input className="pl-9" aria-label="Search sequences" placeholder="Search WC3 or WoW names…" value={query} onChange={(event) => setQuery(event.target.value)} />
        </div>
        <div className="flex h-8 shrink-0 items-stretch gap-2">
          <Button size="icon" className="h-full w-8 shrink-0 box-border" variant={selected.length === metadata.sequences.length ? 'default' : 'outline'} aria-pressed={selected.length === metadata.sequences.length} aria-label={selected.length === metadata.sequences.length ? 'Deselect all animations' : 'Select all animations'} onClick={() => { setCustom(true); setSelected(selected.length === metadata.sequences.length ? [] : metadata.sequences.map((sequence) => sequence.index)); }}>
            <TooltipHelp trigger={<ListChecks className="h-4 w-4" />} tooltips={selected.length === metadata.sequences.length ? 'Deselect all animations' : 'Select all animations'} />
          </Button>
          <Button size="icon" className="h-full w-8 shrink-0 box-border" variant="outline" aria-label="Use default animations" onClick={() => { setError(''); setCustom(false); setSelected(metadata.sequences.filter((sequence) => sequence.defaultIncluded).map((sequence) => sequence.index)); }}>
            <TooltipHelp trigger={<RotateCcw className="h-4 w-4" />} tooltips="Use default animations" />
          </Button>
          <Button size="sm" className="h-full flex-1 min-w-0 box-border border border-transparent" disabled={busy || submitting || (custom && selected.length === 0) || metadata.sequences.length === 0} onClick={() => void submit()}>
            <RefreshCw aria-hidden="true" className={`h-4 w-4 shrink-0 ${busy || submitting ? 'animate-spin' : ''}`} />
            Re-export
          </Button>
        </div>
        <p className="text-xs text-[hsl(var(--viewer-muted))]">{selected.length} of {metadata.sequences.length} selected · {custom ? 'Custom' : 'Default filters'}</p>
        <div ref={listRef} onScroll={(event) => { scrollPosition.current = event.currentTarget.scrollTop; }} className="overflow-y-auto overflow-x-hidden flex-1 min-h-0 min-w-0">
          {sequences.map((sequence) => {
            const playback = playbackSequences.find((item) => item.sourceIndices?.includes(sequence.index));
            return <div key={sequence.index} className={`flex gap-2 px-1 py-2 border-b border-[hsl(var(--viewer-divider))] ${playing === sequence.index ? 'bg-[hsl(var(--viewer-item-active))]' : 'hover:bg-[hsl(var(--viewer-item-hover))]'}`}>
              <Checkbox className="shrink-0" aria-label={`Include ${sequence.wc3Name}, ${sequence.wowName}, variant ${sequence.wowVariant}`} checked={selected.includes(sequence.index)} onCheckedChange={(checked) => {
                setCustom(true);
                setSelected((previous) => (checked === true ? [...previous, sequence.index] : previous.filter((index) => index !== sequence.index)));
              }} />
              <button type="button" className="text-left flex-1 min-w-0 text-sm whitespace-normal [overflow-wrap:anywhere]" onClick={() => {
                if (playback) {
                  onPreview(playback.name); setPlaying(sequence.index);
                  setNotice((playback.sourceIndices?.length ?? 0) > 1 ? 'Playing combined attack preview.' : `Playing ${sequence.wc3Name}.`);
                } else setNotice('Not in current export — select and re-export to preview.');
              }}>
                <span className={`block ${playback ? 'font-semibold' : 'font-normal text-[hsl(var(--viewer-muted))]'}`}>{sequence.wc3Name}</span>
                <span className="block text-xs text-[hsl(var(--viewer-muted))]">{(sequence.duration / 1000).toFixed(2)} s - {sequence.wowName}{metadata.sequences.filter((item) => item.animationID === sequence.animationID).length > 1 ? ` (${sequence.wowVariant})` : ''}</span>
              </button>
            </div>;
          })}
          {sequences.length === 0 && <p className="text-sm text-[hsl(var(--viewer-muted))]">{metadata.sequences.length ? 'No matching sequences.' : 'This model has no source sequences.'}</p>}
        </div>
        {notice && <p role="status" className={`text-xs ${notice.startsWith('Not in current export') ? 'text-red-400' : 'text-[hsl(var(--viewer-muted))]'}`}>{notice}</p>}
      </>}
    </section>
  );
}

export function SequenceEditButton({ editing, busy, onClick }: { editing: boolean; busy: boolean; onClick: () => void }) {
  return <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0 bg-white/5 hover:bg-white/10 text-[hsl(var(--viewer-sidebar-fg))]" disabled={busy} aria-label={editing ? 'Close animation editing' : 'Edit animations'} aria-pressed={editing} onClick={onClick}>
    <TooltipHelp trigger={editing ? <X className="h-4 w-4" /> : <Pencil className="h-4 w-4" />} tooltips={editing ? 'Close animation editing' : 'Select animations to export'} />
  </Button>;
}
