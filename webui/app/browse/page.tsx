import type { Metadata } from 'next';

import BrowseModelPage from '@/components/browse-model';

export const metadata: Metadata = {
  title: 'Browse Models',
};

export default function Page() {
  return <BrowseModelPage />;
}
