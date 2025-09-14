import './globals.css';

import type { Metadata } from 'next';

import SiteHeader from '@/components/common/site-header';
import { ServerConfigProvider } from '@/components/server-config';

export const metadata: Metadata = {
  title: 'Huy\'s wow-converter',
  description: 'Easily export WoW NPC models into Warcraft 3 MDL/MDX',
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="en">
      <head>
        <link rel="icon" href="/favicon.jpg" />
      </head>
      <body style={{ height: 'calc(100vh - 57px)' }}>
        <ServerConfigProvider>
          <SiteHeader />
          {children}
        </ServerConfigProvider>
      </body>
    </html>
  );
}
