'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

export default function SiteHeader() {
  const pathname = usePathname();

  const activeTab = pathname.startsWith('/browse') ? 'browse' : pathname.startsWith('/recents') ? 'recents' : 'export';

  return (
    <div className="sticky top-0 z-50 bg-white/70 backdrop-blur border-b border-gray-200">
      <div className="max-w-6xl mx-auto px-2 py-2">
        {/* <div className="text-center">
          <Link href="/" className="block">
            <h1 className="text-4xl font-bold text-gray-900">Huy's WOW-CONVERTER</h1>
          </Link>
          <p className="text-lg text-gray-600">Easily export WoW NPC models into Warcraft 3 MDL/MDX</p>
        </div> */}

        <div className="flex gap-4">
          <Tabs value={activeTab}>
            <TabsList className="gap-1">
              <TabsTrigger value="export" asChild>
                <Link href="/">Character Export</Link>
              </TabsTrigger>
              <TabsTrigger value="recents" asChild>
                <Link href="/recents">Recent Exports</Link>
              </TabsTrigger>
              <TabsTrigger value="browse" asChild>
                <Link href="/browse">Browse Models</Link>
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
      </div>
    </div>
  );
}
