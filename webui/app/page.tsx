import type { Metadata } from 'next';

import { CharacterConverter } from '@/components/character-converter';

export const metadata: Metadata = {
  title: 'Character Export',
};

export default function Page() {
  return <CharacterConverter />;
}
