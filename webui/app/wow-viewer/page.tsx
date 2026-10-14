import type { Metadata } from 'next';

import WowViewer from './wow-viewer';

export const metadata: Metadata = { title: 'WoW Viewer' };

export default function Page() {
  return <WowViewer />;
}
