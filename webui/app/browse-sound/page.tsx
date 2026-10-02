import type { Metadata } from 'next';

import BrowseSoundPage from '@/components/browse-sound';

export const metadata: Metadata = {
  title: 'Browse Sounds',
};

export default function Page() {
  return <BrowseSoundPage />;
}
