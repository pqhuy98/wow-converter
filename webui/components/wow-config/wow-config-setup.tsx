'use client';

import {
  CheckCircle2, FolderOpen, Globe, HardDrive, Loader2,
} from 'lucide-react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  useCallback, useEffect, useMemo, useRef, useState,
} from 'react';

import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import {
  Card, CardContent, CardDescription, CardHeader, CardTitle,
} from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select';
import {
  Tabs, TabsContent, TabsList, TabsTrigger,
} from '@/components/ui/tabs';
import { useServerConfig } from '@/components/server-config';

import type { CascBuildSummary, WowConfigStatus } from './wow-config-context';
import { useWowConfig } from './wow-config-context';
import {
  normalizeInstallDirectory,
  readStoredInstallDirectory,
  readStoredProduct,
  writeStoredInstallDirectory,
  writeStoredProduct,
} from './wow-config-settings';

type GameOption = { product: string; label: string };

function gameOptionsFromBuilds(
  builds: CascBuildSummary[],
  catalog: WowConfigStatus['products'],
): GameOption[] {
  const seen = new Set<string>();
  const options: GameOption[] = [];

  for (const build of builds) {
    if (seen.has(build.Product)) continue;
    seen.add(build.Product);

    const entry = catalog.find((p) => p.product === build.Product);
    const title = entry?.title ?? build.Product;
    const version = build.Version || build.VersionsName;
    options.push({
      product: build.Product,
      label: version ? `${title} (${version})` : title,
    });
  }

  return options;
}

export function WowConfigSetup() {
  const router = useRouter();
  const { status, refresh } = useWowConfig();
  const { isSharedHosting } = useServerConfig();

  const [mode, setMode] = useState<'local' | 'remote'>(status.config?.mode ?? 'local');
  const [installDirectory, setInstallDirectory] = useState(() => normalizeInstallDirectory(
    status.config?.mode === 'local'
      ? status.config.installDirectory
      : readStoredInstallDirectory(),
  ));
  const [regionTag, setRegionTag] = useState(
    status.config?.mode === 'remote' ? status.config.regionTag : (status.regions[0] ?? 'us'),
  );
  const [product, setProduct] = useState<string | null>(() => (
    status.config?.product ?? readStoredProduct()
  ));
  const [builds, setBuilds] = useState<CascBuildSummary[]>([]);
  const [scanning, setScanning] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadSuccess, setLoadSuccess] = useState(false);
  const [changingSource, setChangingSource] = useState(false);
  const [pickingFolder, setPickingFolder] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const scanGeneration = useRef(0);

  const gameOptions = useMemo(
    () => (builds.length > 0 ? gameOptionsFromBuilds(builds, status.products) : []),
    [builds, status.products],
  );

  const canAutoScan = mode === 'local'
    ? installDirectory.trim().length > 0
    : regionTag.length > 0;

  const resetSelectionForm = useCallback(() => {
    setMode('local');
    setInstallDirectory(readStoredInstallDirectory());
    setRegionTag(status.regions[0] ?? 'us');
    setProduct(readStoredProduct());
    setBuilds([]);
    setError(null);
    setLoadSuccess(false);
  }, [status.regions]);

  const scanBuilds = useCallback(async () => {
    if (status.cascLoaded) return;
    if (mode === 'local' && !installDirectory.trim()) {
      setBuilds([]);
      setProduct(null);
      return;
    }

    const generation = ++scanGeneration.current;
    setError(null);
    setScanning(true);
    setProduct(null);

    try {
      const endpoint = mode === 'local' ? '/api/wow-config/discover-local' : '/api/wow-config/discover-remote';
      const body = mode === 'local'
        ? { installDirectory: installDirectory.trim() }
        : { regionTag };
      const res = await fetch(endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      const json = await res.json();
      if (generation !== scanGeneration.current) return;
      if (!res.ok) throw new Error(json.error ?? 'Could not find games');
      const nextBuilds = json.builds as CascBuildSummary[];
      setBuilds(nextBuilds);
      if (nextBuilds.length > 0) {
        const stored = readStoredProduct();
        if (stored && nextBuilds.some((b) => b.Product === stored)) {
          setProduct(stored);
        }
      }
      if (nextBuilds.length === 0) {
        setError('No WoW games were found. Check the folder or region and try again.');
      }
    } catch (e) {
      if (generation !== scanGeneration.current) return;
      setError((e as Error).message);
      setBuilds([]);
    } finally {
      if (generation === scanGeneration.current) setScanning(false);
    }
  }, [installDirectory, mode, regionTag, status.cascLoaded]);

  useEffect(() => {
    if (status.cascLoaded) setError(null);
  }, [status.cascLoaded]);

  useEffect(() => {
    writeStoredInstallDirectory(installDirectory);
  }, [installDirectory]);

  useEffect(() => {
    if (product) writeStoredProduct(product);
  }, [product]);

  useEffect(() => {
    if (status.cascLoaded) return undefined;
    if (!canAutoScan) {
      scanGeneration.current += 1;
      setBuilds([]);
      setProduct(null);
      setScanning(false);
      return undefined;
    }

    const delay = mode === 'local' ? 400 : 0;
    const timer = setTimeout(() => { void scanBuilds(); }, delay);
    return () => clearTimeout(timer);
  }, [canAutoScan, mode, installDirectory, regionTag, scanBuilds, status.cascLoaded]);

  const pickFolder = async () => {
    setError(null);
    setPickingFolder(true);
    try {
      const res = await fetch('/api/wow-config/pick-local-folder', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          installDirectory: installDirectory.trim() || undefined,
        }),
      });
      const json = await res.json();
      if (!res.ok) throw new Error(json.error ?? 'Could not open folder picker');
      if (json.cancelled) return;
      if (typeof json.installDirectory === 'string') {
        setInstallDirectory(normalizeInstallDirectory(json.installDirectory));
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setPickingFolder(false);
    }
  };

  const loadWoW = async () => {
    if (status.cascLoaded) return;
    if (!product) {
      setError('Choose which WoW game to use.');
      return;
    }
    setError(null);
    setLoading(true);
    try {
      const res = await fetch('/api/wow-config/apply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          mode,
          installDirectory: mode === 'local' ? installDirectory.trim() : undefined,
          regionTag: mode === 'remote' ? regionTag : undefined,
          product,
        }),
      });
      const json = await res.json();
      if (!res.ok) throw new Error(json.error ?? 'Failed to load');
      setLoadSuccess(true);
      await refresh();
      await new Promise((resolve) => { setTimeout(resolve, 1000); });
      router.replace('/');
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const changeSource = async () => {
    setError(null);
    setChangingSource(true);
    try {
      const res = await fetch('/api/wow-config/reset', { method: 'POST' });
      const json = await res.json();
      if (!res.ok) throw new Error(json.error ?? 'Could not change installation source');
      resetSelectionForm();
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setChangingSource(false);
    }
  };

  const gamePlaceholder = (() => {
    if (scanning) return 'Looking for games…';
    if (!canAutoScan) {
      return mode === 'local' ? 'Select an installation folder…' : 'Select a region…';
    }
    if (gameOptions.length === 0) return 'No games found';
    return 'Select a game…';
  })();

  if (!status.wowDataServerReachable) {
    return (
      <Card className="mx-auto max-w-xl">
        <CardHeader>
          <CardTitle>WoW is not ready yet</CardTitle>
          <CardDescription>
            Start wow-converter and try again.
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  if (isSharedHosting) {
    return (
      <Card className="mx-auto max-w-xl">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            {status.cascLoaded ? (
              <CheckCircle2 className="h-5 w-5 text-green-500" />
            ) : (
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            )}
            {status.cascLoaded ? 'WoW is loaded' : 'Loading WoW'}
          </CardTitle>
          <CardDescription>
            {status.cascLoaded
              ? status.cascInfo?.buildName
              : 'WoW data is managed by the server on shared hosting.'}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {status.config && (
            <div className="rounded-md border p-3 text-sm space-y-1">
              <div className="font-medium">Server source</div>
              {status.config.mode === 'local' ? (
                <div className="text-muted-foreground break-all">{status.config.installDirectory}</div>
              ) : (
                <div className="text-muted-foreground">
                  Online —
                  {' '}
                  {status.config.regionTag.toUpperCase()}
                </div>
              )}
            </div>
          )}
          <p className="text-sm text-muted-foreground">
            WoW installation cannot be changed from the web UI in shared hosting mode.
          </p>
          {status.error && !status.cascLoaded && (
            <Alert variant="destructive">
              <AlertDescription>{status.error}</AlertDescription>
            </Alert>
          )}
          <Button asChild>
            <Link href="/">Back</Link>
          </Button>
        </CardContent>
      </Card>
    );
  }

  if (status.cascLoaded && !loadSuccess) {
    return (
      <Card className="mx-auto max-w-xl">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <CheckCircle2 className="h-5 w-5 text-green-500" />
            WoW is loaded
          </CardTitle>
          <CardDescription>
            {status.cascInfo?.buildName}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {status.config && (
            <div className="rounded-md border p-3 text-sm space-y-1">
              <div className="font-medium">Current source</div>
              {status.config.mode === 'local' ? (
                <div className="text-muted-foreground break-all">{status.config.installDirectory}</div>
              ) : (
                <div className="text-muted-foreground">
                  Online —
                  {' '}
                  {status.config.regionTag.toUpperCase()}
                </div>
              )}
            </div>
          )}
          {status.configuredFromEnv && (
            <p className="text-sm text-muted-foreground">
              WoW is configured automatically on startup.
            </p>
          )}
          {error && !status.cascLoaded && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          <div className="flex flex-wrap gap-2">
            <Button asChild>
              <Link href="/">Back</Link>
            </Button>
            {!isSharedHosting && (
              <Button
                type="button"
                variant="outline"
                onClick={() => { void changeSource(); }}
                disabled={changingSource}
              >
                {changingSource && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Change installation source
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="mx-auto max-w-xl">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <HardDrive className="h-5 w-5" />
          Load WoW
        </CardTitle>
        <CardDescription>
          Select your WoW installation folder or region, then choose which game to use.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <Tabs value={mode} onValueChange={(v) => {
          setMode(v as 'local' | 'remote');
          setProduct(null);
          setBuilds([]);
          setError(null);
        }}
        >
          <TabsList className="grid w-full grid-cols-2">
            <TabsTrigger value="local" className="gap-2">
              <FolderOpen className="h-4 w-4" />
              On this PC
            </TabsTrigger>
            <TabsTrigger value="remote" className="gap-2">
              <Globe className="h-4 w-4" />
              Online
            </TabsTrigger>
          </TabsList>

          <TabsContent value="local" className="space-y-4 pt-4">
            <div className="space-y-2">
              <Label htmlFor="install-dir">WoW installation folder</Label>
              <div className="flex gap-2">
                <Input
                  id="install-dir"
                  value={installDirectory}
                  onChange={(e) => setInstallDirectory(normalizeInstallDirectory(e.target.value))}
                  className="flex-1 font-mono text-sm"
                  spellCheck={false}
                  autoComplete="off"
                />
                <Button
                  type="button"
                  variant="outline"
                  className="shrink-0 gap-2"
                  onClick={() => { void pickFolder(); }}
                  disabled={pickingFolder || scanning || loading}
                >
                  {pickingFolder ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <FolderOpen className="h-4 w-4" />
                  )}
                  Browse…
                </Button>
              </div>
            </div>
          </TabsContent>

          <TabsContent value="remote" className="space-y-4 pt-4">
            <div className="space-y-2">
              <Label>Region</Label>
              <Select value={regionTag} onValueChange={setRegionTag}>
                <SelectTrigger>
                  <SelectValue placeholder="Select region" />
                </SelectTrigger>
                <SelectContent>
                  {status.regions.map((r) => (
                    <SelectItem key={r} value={r}>{r.toUpperCase()}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </TabsContent>
        </Tabs>

        <div className="space-y-2">
          <Label className="flex items-center gap-2">
            Game
            {scanning && <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />}
          </Label>
          <Select
            value={product ?? undefined}
            onValueChange={setProduct}
            disabled={gameOptions.length === 0 || scanning || loading}
          >
            <SelectTrigger>
              <SelectValue placeholder={gamePlaceholder} />
            </SelectTrigger>
            <SelectContent>
              {gameOptions.map((g) => (
                <SelectItem key={g.product} value={g.product}>
                  {g.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            onClick={() => { void loadWoW(); }}
            disabled={loading || loadSuccess || scanning || !product || gameOptions.length === 0}
            className={loadSuccess ? 'bg-green-600 text-white hover:bg-green-600' : undefined}
          >
            {loadSuccess ? (
              <>
                <CheckCircle2 className="mr-2 h-4 w-4" />
                Loaded
              </>
            ) : loading ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Load
              </>
            ) : (
              'Load'
            )}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
