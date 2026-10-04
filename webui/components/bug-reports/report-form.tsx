'use client';

import { Loader2 } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ExportReportMetadata } from '@/lib/models/export-character.model';

import { ReportImages } from './report-images';

interface CaptureStatus {
  id: string
  status: string
  error?: string
  result?: { images: string[] }
}

async function reportFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init);
  const text = await response.text();
  let body: T & { error?: string };
  try { body = JSON.parse(text); } catch { throw new Error('The report server is unavailable. Please try again later.'); }
  if (!response.ok) throw new Error(body.error ?? 'Could not complete the report. Please try again.');
  return body;
}

export function ReportForm({
  exportId, metadata, modelPath, currentSequence,
}: {
  exportId: string; metadata: ExportReportMetadata; modelPath: string; currentSequence: number;
}) {
  const [open, setOpen] = useState(false);
  const [sequence, setSequence] = useState(currentSequence);
  const [notes, setNotes] = useState('');
  const [quota, setQuota] = useState<{remaining: number; resetAt: number} | null>(null);
  const checkedQuota = useRef(false);
  const [capture, setCapture] = useState<CaptureStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [url, setURL] = useState('');
  const sequences = metadata.models.find((model) => model.path === modelPath)?.sequences ?? [];
  const capturing = capture?.status === 'pending' || capture?.status === 'processing';
  const selected = sequences.find((item) => item.index === sequence);

  useEffect(() => {
    if (!open || checkedQuota.current) return;
    checkedQuota.current = true;
    void reportFetch<{remaining: number; resetAt: number}>('/api/bug-report-captures/quota').then(setQuota).catch(() => {
      // Submission enforces the quota even when this optional preflight is unavailable.
    });
  }, [open]);
  useEffect(() => {
    if (!capturing || !capture) return undefined;
    const controller = new AbortController();
    const timer = setInterval(() => {
      void reportFetch<CaptureStatus>(`/api/bug-report-captures/${capture.id}`, { signal: controller.signal }).then((status) => {
        setCapture(status);
        if (status.status === 'failed') setError(status.error ?? 'Screenshots failed. Please retake them.');
      }).catch((failure) => {
        if (controller.signal.aborted) return;
        setError(failure instanceof Error ? failure.message : 'Screenshot capture failed.');
        setCapture(null);
      });
    }, 1000);
    return () => { controller.abort(); clearInterval(timer); };
  }, [capture?.id, capturing]);

  const prepare = async () => {
    setError(''); setBusy(true); setCapture(null);
    try {
      setCapture(await reportFetch('/api/bug-report-captures', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ exportId, model: modelPath, sequenceIndex: sequence }),
      }));
    } catch (failure) { setError(failure instanceof Error ? failure.message : 'Could not prepare screenshots.'); } finally { setBusy(false); }
  };

  const submit = async () => {
    if (!capture || capture.status !== 'done') return;
    setBusy(true); setError('');
    try {
      const accepted = await reportFetch<{url: string}>(`/api/bug-report-captures/${capture.id}/submit`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ notes }),
      });
      setURL(accepted.url);
    } catch (failure) { setError(failure instanceof Error ? failure.message : 'Could not submit your report.'); } finally { setBusy(false); }
  };

  return <>
    <Button variant="outline" onClick={() => { setSequence(currentSequence); setCapture(null); setError(''); setOpen(true); }}>Doesn&apos;t match Wowhead?</Button>
    <Dialog open={open} onOpenChange={(value) => { if (!busy && !capturing) setOpen(value); }}>
      <DialogContent className="max-w-5xl max-h-[95vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Report a Wowhead mismatch</DialogTitle>
          <DialogDescription>Help us improve the accuracy of the converter by submitting a report of a mismatch.</DialogDescription>
        </DialogHeader>
        {url ? <div role="status" className="space-y-3"><p>Your report was submitted successfully.</p><a href={url} target="_blank" rel="noreferrer" className="underline text-primary">View your submission</a></div> : <div className="space-y-4">
          {quota?.remaining === 0 && <p role="alert" className="text-sm text-destructive">You are out of today’s submission quota. Please try again after {new Date(quota.resetAt).toLocaleString()}.</p>}
          <div className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] gap-3 items-end">
          <div className="space-y-1">
            <Label htmlFor="report-sequence">Model sequence</Label>
            <select id="report-sequence" className="w-full border rounded bg-background p-2" value={sequence} disabled={busy || capturing} onChange={(event) => { setSequence(Number(event.target.value)); setCapture(null); setError(''); }}>
              {sequences.map((item) => <option key={item.index} value={item.index}>{item.name}{item.wowName ? ` → ${item.wowName}` : ' (no Wowhead counterpart)'}</option>)}
            </select>
            {selected && !selected.wowName && <p className="text-sm text-destructive">This generated sequence has no Wowhead counterpart. Choose another sequence to capture both viewers.</p>}
          </div>
          <div className="space-y-1">
            <Label htmlFor="report-notes">What looks wrong? (required)</Label>
            <Input id="report-notes" value={notes} maxLength={8000} disabled={busy} onChange={(event) => setNotes(event.target.value)} placeholder="Missing parts, incorrect textures, animation…" />
          </div>
          </div>
          <div className="flex flex-wrap gap-2">
          <Button variant="outline" disabled={busy || capturing || !selected?.wowName} onClick={() => void prepare()}>
            {(capturing || (busy && !capture)) && <Loader2 className="h-4 w-4 animate-spin" />}
            {capturing || (busy && !capture) ? 'Preparing screenshots…' : capture?.status === 'done' ? 'Retake screenshots' : 'Prepare screenshots'}
          </Button>
          <Button disabled={busy || capturing || capture?.status !== 'done' || !notes.trim() || quota?.remaining === 0} onClick={() => void submit()}>{busy && capture?.status === 'done' ? 'Submitting…' : 'Confirm submission'}</Button>
          </div>
          {capture?.status === 'done' && capture.result && <ReportImages images={capture.result.images} />}
          {error && <p role="alert" className="text-destructive text-sm">{error}</p>}
          <p className="text-xs text-muted-foreground">Only screenshots, your notes and export settings are sent. Anyone with your submission link can view it.</p>
        </div>}
      </DialogContent>
    </Dialog>
  </>;
}
