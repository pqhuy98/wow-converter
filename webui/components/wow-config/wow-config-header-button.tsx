'use client';

import { HardDrive } from 'lucide-react';
import Link from 'next/link';
import { useMemo } from 'react';

import { Button } from '@/components/ui/button';

import { useServerConfig } from '../server-config';
import type { WowConfigStatus } from './wow-config-context';
import { useWowConfig } from './wow-config-context';

function sourceLabel(status: WowConfigStatus): 'local' | 'CDN' {
  if (status.config?.mode === 'remote') return 'CDN';
  if (status.config?.mode === 'local') return 'local';
  if (status.cascInfo?.type.includes('Remote')) return 'CDN';
  return 'local';
}

function headerLabel(status: WowConfigStatus): string {
  if (!status.cascLoaded) return 'Load WoW';

  const version = status.cascInfo?.build.Version
    ?? status.cascInfo?.build.VersionsName
    ?? status.cascInfo?.buildName
    ?? 'WoW';

  return `${version} (${sourceLabel(status)})`;
}

export function WowConfigHeaderButton() {
  const { status } = useWowConfig();
  const { isSharedHosting } = useServerConfig();
  const label = useMemo(() => headerLabel(status), [status]);

  if (isSharedHosting) {
    return (
      <Button
        variant="outline"
        size="sm"
        disabled
        className="h-9 max-w-[min(100vw-8rem,14rem)] gap-1.5 px-2.5 font-normal"
        title="WoW installation is fixed on shared hosting and cannot be changed here"
      >
        <HardDrive className="h-4 w-4 shrink-0" />
        <span className="truncate text-xs">{label}</span>
      </Button>
    );
  }

  return (
    <Button variant="outline" size="sm" asChild className="h-9 max-w-[min(100vw-8rem,14rem)] gap-1.5 px-2.5 font-normal">
      <Link href="/setup" title={label}>
        <HardDrive className="h-4 w-4 shrink-0" />
        <span className="truncate text-xs">{label}</span>
      </Link>
    </Button>
  );
}
