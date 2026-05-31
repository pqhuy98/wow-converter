import type { Metadata } from 'next';

import ViewerPage from './viewer-content';

export const metadata: Metadata = {
  title: 'Model Viewer',
};

export default function Page() {
  return <ViewerPage />;
}
