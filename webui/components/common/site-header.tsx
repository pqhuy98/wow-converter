'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

import { useServerConfig } from '../server-config';
import { ThemeToggle } from './theme-toggle';

export default function SiteHeader() {
  const pathname = usePathname();
  const { isSharedHosting } = useServerConfig();

  const activeTab = pathname.startsWith('/browse')
    ? 'browse'
    : pathname.startsWith('/recents')
      ? 'recents'
      : pathname.startsWith('/maps') && !isSharedHosting
        ? 'maps'
        : 'export';

  return (
    <div className="sticky top-0 z-10 bg-background/80 backdrop-blur border-b border-border">
      <div className="max-w-6xl mx-auto px-2 py-2">
        <div className="flex gap-4 items-center justify-center">
          <Tabs value={activeTab}>
            <TabsList className="">
              <TabsTrigger value="export" asChild>
                <Link href="/">Character Export</Link>
              </TabsTrigger>
              <TabsTrigger value="recents" asChild>
                <Link href="/recents">Recent Exports</Link>
              </TabsTrigger>
              <TabsTrigger value="browse" asChild>
                <Link href="/browse">Browse Models</Link>
              </TabsTrigger>
              {!isSharedHosting && <TabsTrigger value="maps" asChild>
                <Link href="/maps">Maps</Link>
              </TabsTrigger>}
            </TabsList>
          </Tabs>
          <div className="shrink-0 absolute right-1">
            <ThemeToggle />
          </div>
        </div>
      </div>
    </div>
  );
}
