'use client';

import { Loader2 } from 'lucide-react';
import { usePathname, useRouter } from 'next/navigation';
import { useEffect } from 'react';

import { useWowConfig } from './wow-config-context';

/** Send users to /setup until WoW data is loaded. */
export function WowConfigGate({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { status, isReady } = useWowConfig();

  const redirectToSetup = isReady && pathname !== '/setup' && !status.cascLoaded;

  useEffect(() => {
    if (redirectToSetup) {
      router.replace('/setup');
    }
  }, [redirectToSetup, router]);

  if (!isReady || redirectToSetup) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return children;
}
