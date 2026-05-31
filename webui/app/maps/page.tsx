import type { Metadata } from 'next';

import MapViewer from '@/components/map-viewer';

export const metadata: Metadata = {
  title: 'Maps',
};

export default function MapsPage() {
  return <MapViewer />;
}


