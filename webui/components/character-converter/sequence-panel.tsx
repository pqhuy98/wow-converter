'use client';

import { ListChecks, Pencil, RefreshCw, RotateCcw, Search, X } from 'lucide-react';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';

import { TooltipHelp } from '@/components/common/tooltip-help';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import {
  AnimationEdits, AnimationRename, Character, ModelMetadata, ReportSequence,
} from '@/lib/models/export-character.model';

export function SequencePanel({
  character, buildKey, skinId, playbackSequences, busy, initialRenames, onPreview, onApply, onChange,
}: {
  character: Character
  buildKey: string
  skinId?: string
  playbackSequences: ReportSequence[]
  busy: boolean
  onPreview: (name: string, index: number) => void
  initialRenames?: AnimationRename[]
  onApply: (edits: AnimationEdits) => Promise<boolean>
  onChange?: (edits: AnimationEdits) => void
}) {
  const [metadata, setMetadata] = useState<ModelMetadata>();
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<number[]>([]);
  const [custom, setCustom] = useState(!!character.sequenceSelection);
  const [submitting, setSubmitting] = useState(false);
  const [playing, setPlaying] = useState<number>();
  const [notice, setNotice] = useState('');
  const [renames, setRenames] = useState<AnimationRename[]>(initialRenames ?? []);
  const [renaming, setRenaming] = useState<number>();
  const previousName = useRef<string>();
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const listRef = useRef<HTMLDivElement>(null);
  const scrollPosition = useRef(0);

  const edits = (): AnimationEdits => ({
    selection: custom && metadata && (selected.length !== metadata.sequences.filter((sequence) => sequence.defaultIncluded).length || metadata.sequences.some((sequence) => selected.includes(sequence.index) !== sequence.defaultIncluded)) ? { fileDataID: metadata.fileDataID, buildKey: metadata.buildKey, indices: [...selected].sort((a, b) => a - b) } : undefined,
    renames: renames.map((rename) => ({ ...rename, newWc3Name: rename.newWc3Name.trim() })).filter((rename) => rename.newWc3Name),
  });
  useEffect(() => {
    if (metadata) onChangeRef.current?.(edits());
  }, [metadata, custom, selected, renames]);

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
      if (!await onApply(edits())) setError('Export was not accepted. Your edits are preserved; correct the export settings and try again.');
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : 'Export could not be submitted');
    } finally { setSubmitting(false); }
  };

  const sequences = metadata?.sequences.filter((sequence) => `${renames.find((rename) => rename.wowName === sequence.wowName && rename.wowVariant === sequence.wowVariant)?.newWc3Name ?? sequence.wc3Name} ${sequence.wc3Name} ${sequence.wowName} ${sequence.wowVariant}`.toLowerCase().includes(query.toLowerCase())) ?? [];
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
          <Button size="icon" className="h-full w-8 shrink-0 box-border" variant="outline" aria-label="Use default animations" onClick={() => { setError(''); setCustom(false); setRenames([]); setRenaming(undefined); setSelected(metadata.sequences.filter((sequence) => sequence.defaultIncluded).map((sequence) => sequence.index)); }}>
            <TooltipHelp trigger={<RotateCcw className="h-4 w-4" />} tooltips="Use default animations" />
          </Button>
          <Button size="sm" className="h-full flex-1 min-w-0 box-border border border-transparent" disabled={busy || submitting || (custom && selected.length === 0) || metadata.sequences.length === 0} onClick={() => void submit()}>
            <RefreshCw aria-hidden="true" className={`h-4 w-4 shrink-0 ${busy || submitting ? 'animate-spin' : ''}`} />
            Re-export
          </Button>
        </div>
        <p className="text-xs text-[hsl(var(--viewer-muted))]">{selected.length} of {metadata.sequences.length} selected · {custom || renames.length ? 'Custom' : 'Default filters'}</p>
        <div ref={listRef} onScroll={(event) => { scrollPosition.current = event.currentTarget.scrollTop; }} className="overflow-y-auto overflow-x-hidden flex-1 min-h-0 min-w-0">
          {sequences.map((sequence) => {
            const playback = playbackSequences.find((item) => item.sourceIndices?.includes(sequence.index));
            const name = renames.find((rename) => rename.wowName === sequence.wowName && rename.wowVariant === sequence.wowVariant)?.newWc3Name ?? sequence.wc3Name;
            const changeName = (newWc3Name: string | undefined) => setRenames((previous) => {
              const others = previous.filter((rename) => rename.wowName !== sequence.wowName || rename.wowVariant !== sequence.wowVariant);
              return newWc3Name === undefined ? others : [...others, { wowName: sequence.wowName, wowVariant: sequence.wowVariant, newWc3Name }];
            });
            const preview = () => {
              if (playback) {
                onPreview(playback.name, playback.index); setPlaying(sequence.index);
                setNotice((playback.sourceIndices?.length ?? 0) > 1 ? 'Playing combined attack preview.' : `Playing ${name}.`);
              } else setNotice('Not in current export — select and re-export to preview.');
            };
            return <div key={sequence.index} className={`flex gap-2 px-1 py-2 border-b border-[hsl(var(--viewer-divider))] ${playing === sequence.index ? 'bg-[hsl(var(--viewer-item-active))]' : 'hover:bg-[hsl(var(--viewer-item-hover))]'}`}>
              <Checkbox className="shrink-0" aria-label={`Include ${sequence.wc3Name}, ${sequence.wowName}, variant ${sequence.wowVariant}`} checked={selected.includes(sequence.index)} onCheckedChange={(checked) => {
                setCustom(true);
                setSelected((previous) => (checked === true ? [...previous, sequence.index] : previous.filter((index) => index !== sequence.index)));
              }} />
              <div className="flex-1 min-w-0 text-sm">
                <div className="flex items-center gap-1">
                  {renaming === sequence.index ? <Input autoFocus className="h-5 min-w-0 flex-1 px-0 py-0 text-sm font-semibold" aria-label={`Rename ${sequence.wowName} (${sequence.wowVariant})`} value={name} onChange={(event) => changeName(event.target.value)} onFocus={(event) => event.target.select()} onBlur={() => { changeName(name.trim() || undefined); setRenaming(undefined); }} onKeyDown={(event) => { if (event.key === 'Enter') event.currentTarget.blur(); if (event.key === 'Escape') { changeName(previousName.current); setRenaming(undefined); } }} /> : <button type="button" onClick={preview} className={`h-5 flex-1 min-w-0 truncate text-left ${playback ? 'font-semibold' : 'font-normal text-[hsl(var(--viewer-muted))]'}`}>{name}</button>}
                  <Button size="icon" variant="ghost" className="h-5 w-5 shrink-0 text-[hsl(var(--viewer-muted))]" aria-label={`Rename ${sequence.wc3Name}`} title="Rename animation" onClick={() => { previousName.current = renames.find((rename) => rename.wowName === sequence.wowName && rename.wowVariant === sequence.wowVariant)?.newWc3Name; setRenaming(sequence.index); }}><Pencil className="h-3 w-3" /></Button>
                </div>
                <button type="button" onClick={preview} className="block text-left text-xs text-[hsl(var(--viewer-muted))] whitespace-normal [overflow-wrap:anywhere]">{(sequence.duration / 1000).toFixed(2)} s - {sequence.wowName}{metadata.sequences.filter((item) => item.animationID === sequence.animationID).length > 1 ? ` (${sequence.wowVariant})` : ''}</button>
              </div>
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
