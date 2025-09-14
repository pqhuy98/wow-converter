import './globals.css';

import type { Metadata } from 'next';

import SiteHeader from '@/components/common/site-header';
import { ServerConfigProvider } from '@/components/server-config';
import { ThemeProvider } from '@/components/theme-provider';

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
    <html lang="en" suppressHydrationWarning>
      <head>
        <link rel="icon" href="/favicon.jpg" />
      </head>
      <body style={{ height: 'calc(100vh - 57px)' }}>
        <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
          <ServerConfigProvider>
            <SiteHeader />
            {children}
          </ServerConfigProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
