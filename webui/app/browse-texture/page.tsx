import type { Metadata } from 'next';

import BrowseTexturePage from '@/components/browse-texture';

export const metadata: Metadata = {
  title: 'Browse Textures',
};

export default function Page() {
  return <BrowseTexturePage />;
}
