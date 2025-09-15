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
      <body className="min-h-screen bg-[radial-gradient(80%_80%_at_50%_50%,hsl(var(--background))_0%,hsl(var(--background-alt))_45%,hsl(var(--background))_100%)]">
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
