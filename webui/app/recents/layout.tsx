import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Recent Exports',
};

export default function RecentsLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return children;
}
