'use client';

import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { ExportRequest, ReportSequence } from '@/lib/models/export-character.model';

import { ReportImages } from './report-images';

interface Report {
  id: string
  wowheadUrl: string
  modelName?: string
  request: ExportRequest
  sequence: ReportSequence
  notes: string
  status: 'open' | 'fixed' | 'declined'
  gitSha: string
  product: string
  buildKey: string
  exportedAt: number
  createdAt: number
  updatedAt: number
  images: string[]
}

interface ReportList { items: Report[]; total: number; page: number; pageSize: number }

const passwordKey = 'bug-report-admin-password';

export function BugReports() {
  const [ready, setReady] = useState(false);
  const [id, setID] = useState('');
  const [password, setPassword] = useState('');
  const [passwordDraft, setPasswordDraft] = useState('');
  const [loginOpen, setLoginOpen] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [items, setItems] = useState<Report[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');
  const [search, setSearch] = useState('');
  const [searchDraft, setSearchDraft] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    setID(window.location.pathname.split('/').filter(Boolean)[1] ?? '');
    try { setPassword(localStorage.getItem(passwordKey) ?? ''); } catch { /* Storage can be disabled. */ }
    setReady(true);
  }, []);

  const request = async <T, >(path: string, init?: RequestInit): Promise<T> => {
    const response = await fetch(path, { ...init, headers: { ...init?.headers, ...(password ? { Authorization: `Bearer ${password}` } : {}) } });
    const body = await response.json();
    if (response.status === 401) {
      try { localStorage.removeItem(passwordKey); } catch { /* Storage can be disabled. */ }
      setPassword(''); setAuthenticated(false); setLoginOpen(true);
    }
    if (!response.ok) throw new Error(body.error ?? 'Could not load bug reports.');
    return body;
  };

  useEffect(() => {
    if (!ready || (!id && !password)) return undefined;
    let cancelled = false;
    setLoading(true); setError('');
    const load = async () => {
      try {
        if (id) { const report = await request<Report>(`/api/bug-reports/${encodeURIComponent(id)}`); if (!cancelled) setItems([report]); } else {
          const query = new URLSearchParams({ page: String(page), status, search });
          const list = await request<ReportList>(`/api/bug-reports?${query.toString()}`);
          if (!cancelled) { setItems(list.items); setTotal(list.total); }
        }
        if (!cancelled && password) {
          setAuthenticated(true); setLoginOpen(false);
          try { localStorage.setItem(passwordKey, password); } catch { /* Storage can be disabled. */ }
        }
      } catch (failure) { if (!cancelled) setError(failure instanceof Error ? failure.message : 'Could not load reports.'); } finally { if (!cancelled) setLoading(false); }
    };
    void load();
    return () => { cancelled = true; };
  }, [ready, id, password, page, status, search, revision]);

  const update = async (report: Report, edit: {status?: string; notes?: string; delete?: boolean}) => {
    try {
      await request(`/api/bug-reports/${report.id}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(edit) });
      if (edit.delete && id) { window.location.assign('/bug-reports'); return; }
      setRevision((value) => value + 1);
    } catch (failure) { setError(failure instanceof Error ? failure.message : 'Could not update the report.'); throw failure; }
  };

  if (!ready) return <main className="max-w-6xl mx-auto p-6">Loading…</main>;
  return <main className="max-w-6xl mx-auto p-6 space-y-6">
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h1 className="text-2xl font-semibold">{id ? 'Bug report' : 'Bug Reports'}</h1>
      <div className="flex gap-2">
        {id && authenticated && <a href="/bug-reports" className="underline self-center">All reports</a>}
        {authenticated ? <Button variant="outline" onClick={() => { try { localStorage.removeItem(passwordKey); } catch { /* Storage can be disabled. */ } setPassword(''); setAuthenticated(false); setItems([]); }}>Log out</Button>
          : id && <Button variant="outline" onClick={() => setLoginOpen((value) => !value)}>Admin login</Button>}
      </div>
    </div>
    {(!id && !authenticated || loginOpen) && <form className="max-w-md flex flex-col gap-3 border rounded p-4" onSubmit={(event) => { event.preventDefault(); setPassword(passwordDraft); setRevision((value) => value + 1); }}>
      <Label htmlFor="report-password">Admin password</Label>
      <Input id="report-password" type="password" autoComplete="current-password" required value={passwordDraft} onChange={(event) => setPasswordDraft(event.target.value)} />
      <Button disabled={loading || !passwordDraft} type="submit">{loading ? 'Checking…' : 'Log in'}</Button>
    </form>}
    {error && <p role="alert" className="text-destructive">{error}</p>}
    {!id && authenticated && <form className="flex flex-wrap items-center gap-3" onSubmit={(event) => { event.preventDefault(); setSearch(searchDraft); setPage(1); }}>
      <Input className="flex-1 min-w-48" aria-label="Search reports" placeholder="Search URL, notes, model settings…" value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} />
      <Button type="submit" variant="outline">Search</Button>
      <select aria-label="Filter by status" className="border rounded p-2 bg-background" value={status} onChange={(event) => { setStatus(event.target.value); setPage(1); }}>
        <option value="">All statuses</option><option value="open">Open</option><option value="fixed">Fixed</option><option value="declined">Declined</option>
      </select>
      <span className="text-sm text-muted-foreground">{total} reports</span>
    </form>}
    {loading && <p role="status">Loading reports…</p>}
    {(!id && authenticated || id) && <div className="space-y-6">
      {items.map((report) => <ReportCard key={`${report.id}:${report.updatedAt}`} report={report} detail={Boolean(id)} admin={authenticated} update={update} />)}
      {!loading && !items.length && !error && <p>No reports found.</p>}
    </div>}
    {!id && authenticated && <div className="flex justify-between items-center">
      <Button variant="outline" disabled={page <= 1 || loading} onClick={() => setPage((value) => value - 1)}>Previous</Button>
      <span>Page {page} of {Math.max(1, Math.ceil(total / 12))}</span>
      <Button variant="outline" disabled={page * 12 >= total || loading} onClick={() => setPage((value) => value + 1)}>Next</Button>
    </div>}
  </main>;
}

function ReportCard({
  report, detail, admin, update,
}: {
  report: Report; detail: boolean; admin: boolean;
  update: (report: Report, edit: {status?: string; notes?: string; delete?: boolean}) => Promise<void>;
}) {
  const [notes, setNotes] = useState(report.notes);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState('');
  const change = async (edit: {status?: string; notes?: string; delete?: boolean}) => {
    setBusy(true);
    try { await update(report, edit); setEditing(false); } catch { /* Parent displays the API error. */ } finally { setBusy(false); }
  };
  const copyPrompt = async () => {
    const origin = window.location.origin;
    const prompt = `Use Cursor skill /debug-wowhead-mismatch to fix this mismatch:\n${report.wowheadUrl}\nWowhead screenshot: ${origin}${report.images[0]}\nWow-converter screenshot: ${origin}${report.images[1]}\n${JSON.stringify({ ...report, images: undefined }, null, 2)}`;
    try { await navigator.clipboard.writeText(prompt); setCopied(true); setCopyError(''); } catch { setCopyError('Could not copy the prompt. Allow clipboard access and try again.'); }
  };
  const character = report.request.character;
  const settings = {
    model: report.modelName, weapon: character.attackTag || 'Auto', attachments: character.attachItems, mount: character.mount, animation: report.sequence.name, wowAnimation: report.sequence.wowName, wowVariant: report.sequence.wowVariant, format: report.request.format, modelVersion: report.request.formatVersion,
  };

  return <article className={`border rounded-lg p-4 space-y-4 ${detail ? '' : 'cursor-pointer hover:border-primary'}`} role={detail ? undefined : 'link'} tabIndex={detail ? undefined : 0}
    onClick={() => { if (!detail) window.location.assign(`/bug-reports/${report.id}`); }}
    onKeyDown={(event) => { if (!detail && event.target === event.currentTarget && event.key === 'Enter') window.location.assign(`/bug-reports/${report.id}`); }}>
    <ReportImages images={report.images} />
    <div className="flex flex-wrap justify-between gap-2">
      <a href={report.wowheadUrl} target="_blank" rel="noreferrer" className="underline text-primary break-all" onClick={(event) => event.stopPropagation()}>{report.wowheadUrl}</a>
      <span className="font-medium capitalize">{report.status}</span>
    </div>
    <p className="whitespace-pre-wrap break-words">{report.notes}</p>
    <pre className="bg-muted p-3 rounded text-xs overflow-x-auto whitespace-pre-wrap break-words">{JSON.stringify(settings, null, 2)}</pre>
    <p className="text-xs text-muted-foreground">Submitted {new Date(report.createdAt).toLocaleString()} · {report.product} · Git {report.gitSha.slice(0, 12)}</p>
    {detail && <dl className="space-y-2 text-sm">
      <dt className="font-semibold">Report ID</dt><dd className="break-all">{report.id}</dd>
      <dt className="font-semibold">Exported</dt><dd>{new Date(report.exportedAt).toLocaleString()}</dd>
      <dt className="font-semibold">Last updated</dt><dd>{new Date(report.updatedAt).toLocaleString()}</dd>
      <dt className="font-semibold">Converter Git SHA</dt><dd className="break-all">{report.gitSha}</dd>
      <dt className="font-semibold">WoW build key</dt><dd className="break-all">{report.buildKey || 'Unknown'}</dd>
      <dt className="font-semibold">Export request</dt><dd><pre className="bg-muted p-3 rounded overflow-x-auto text-xs whitespace-pre-wrap break-words">{JSON.stringify(report.request, null, 2)}</pre></dd>
    </dl>}
    {admin && <div className="flex flex-wrap gap-2" onClick={(event) => event.stopPropagation()}>
      <Button size="sm" disabled={busy || report.status === 'fixed'} onClick={() => void change({ status: 'fixed' })}>Mark fixed</Button>
      <Button size="sm" variant="outline" disabled={busy || report.status === 'declined'} onClick={() => void change({ status: 'declined' })}>Decline</Button>
      {report.status !== 'open' && <Button size="sm" variant="outline" disabled={busy} onClick={() => void change({ status: 'open' })}>Reopen</Button>}
      <Button size="sm" variant="outline" disabled={busy} onClick={() => setEditing((value) => !value)}>Edit notes</Button>
      <Button size="sm" variant="outline" onClick={() => void copyPrompt()}>{copied ? 'Prompt copied' : 'Copy AI prompt'}</Button>
      <Button size="sm" variant="destructive" disabled={busy} onClick={() => { if (window.confirm('Delete this report and both screenshots?')) void change({ delete: true }); }}>Delete</Button>
      {copyError && <p role="alert" className="text-sm text-destructive w-full">{copyError}</p>}
      {editing && <div className="w-full space-y-2"><Textarea aria-label="Edit report notes" maxLength={8000} value={notes} onChange={(event) => setNotes(event.target.value)} /><Button size="sm" disabled={busy || !notes.trim()} onClick={() => void change({ notes })}>Save notes</Button></div>}
    </div>}
  </article>;
}
