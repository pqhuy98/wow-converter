import type { Metadata } from 'next';

import { WowConfigSetup } from '@/components/wow-config/wow-config-setup';

export const metadata: Metadata = {
  title: 'WoW Setup',
};

export default function SetupPage() {
  return (
    <main className="container mx-auto px-4 py-8">
      <WowConfigSetup />
    </main>
  );
}
