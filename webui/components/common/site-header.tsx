'use client';

import {
  Box, Clock, Image, Map, Search,
} from 'lucide-react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

import { useServerConfig } from '../server-config';
import { ThemeToggle } from './theme-toggle';

type ActiveTab = 'export' | 'browse' | 'browse-texture' | 'recents' | 'maps';

function getActiveTab(pathname: string, isSharedHosting: boolean): ActiveTab {
  if (pathname.startsWith('/browse-texture')) {
    return 'browse-texture';
  }
  if (pathname.startsWith('/browse')) {
    return 'browse';
  }
  if (pathname.startsWith('/recents')) {
    return 'recents';
  }
  if (pathname.startsWith('/maps') && !isSharedHosting) {
    return 'maps';
  }
  return 'export';
}

export default function SiteHeader() {
  const pathname = usePathname();
  const { isSharedHosting } = useServerConfig();

  const activeTab = getActiveTab(pathname, isSharedHosting);

  return (
    <div className="sm:sticky sm:top-0 z-50 bg-background/80 backdrop-blur-sm border-b border-border">
      <div className="max-w-6xl mx-auto px-2 sm:px-2 py-2">
        <div className="flex items-center justify-between sm:justify-center w-full">
          {/* Mobile: Custom buttons */}
          <div className="flex gap-1 sm:hidden">
            <Link href="/" className={`p-2 rounded-md ${activeTab === 'export' ? 'bg-primary/20' : 'hover:bg-accent'}`} title="Character Export">
              <Box className="w-5 h-5" />
            </Link>
            <Link href="/browse" className={`p-2 rounded-md ${activeTab === 'browse' ? 'bg-primary/20' : 'hover:bg-accent'}`} title="Browse Models">
              <Search className="w-5 h-5" />
            </Link>
            <Link href="/browse-texture" className={`p-2 rounded-md ${activeTab === 'browse-texture' ? 'bg-primary/20' : 'hover:bg-accent'}`} title="Browse Textures">
              <Image className="w-5 h-5" />
            </Link>
            {!isSharedHosting && (
              <Link href="/maps" className={`p-2 rounded-md ${activeTab === 'maps' ? 'bg-primary/20' : 'hover:bg-accent'}`} title="Maps">
                <Map className="w-5 h-5" />
              </Link>
            )}
            <Link href="/recents" className={`p-2 rounded-md ${activeTab === 'recents' ? 'bg-primary/20' : 'hover:bg-accent'}`} title="Recent Exports">
              <Clock className="w-5 h-5" />
            </Link>
          </div>

          {/* Desktop: Tabs */}
          <Tabs value={activeTab} className="hidden sm:flex sm:flex-none min-w-0">
            <TabsList className="flex h-9 gap-0">
              <TabsTrigger value="export" asChild className="px-3" title="Character Export">
                <Link href="/" className="flex items-center justify-center">
                  <Box className="w-5 h-5" />
                  <span className="ml-2">Export</span>
                </Link>
              </TabsTrigger>
              <TabsTrigger value="browse" asChild className="px-3" title="Browse Models">
                <Link href="/browse" className="flex items-center justify-center">
                  <Search className="w-5 h-5" />
                  <span className="ml-2">Browse</span>
                </Link>
              </TabsTrigger>
              <TabsTrigger value="browse-texture" asChild className="px-3" title="Browse Textures">
                <Link href="/browse-texture" className="flex items-center justify-center">
                  <Image className="w-5 h-5" />
                  <span className="ml-2">Textures</span>
                </Link>
              </TabsTrigger>
              {!isSharedHosting && <TabsTrigger value="maps" asChild className="px-3" title="Maps">
                <Link href="/maps" className="flex items-center justify-center">
                  <Map className="w-5 h-5" />
                  <span className="ml-2">Maps</span>
                </Link>
              </TabsTrigger>}
              <TabsTrigger value="recents" asChild className="px-3" title="Recent Exports">
                <Link href="/recents" className="flex items-center justify-center">
                  <Clock className="w-5 h-5" />
                  <span className="ml-2">Recent</span>
                </Link>
              </TabsTrigger>
            </TabsList>
          </Tabs>
          <div className="shrink-0 sm:absolute sm:right-1 ml-1 sm:ml-0">
            <ThemeToggle />
          </div>
        </div>
      </div>
    </div>
  );
}
