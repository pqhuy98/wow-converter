import os from 'node:os';
import { fileURLToPath } from 'node:url';

// Next.js sets PORT to its own development port.
const apiPort = process.env.API_PORT || '3001';
const isDev = process.env.NODE_ENV === 'development';

function lanDevOrigins() {
  const origins = [];
  for (const addrs of Object.values(os.networkInterfaces())) {
    for (const a of addrs ?? []) {
      if ((a.family === 'IPv4' || a.family === 4) && !a.internal) {
        origins.push(a.address);
      }
    }
  }
  return origins;
}

/** @type {import('next').NextConfig} */
const nextConfig = {
  devIndicators: false,
  // Development requests use rewrites to reach the converter API.
  ...(isDev ? {
    distDir: '.next-dev',
    allowedDevOrigins: ['localhost', '127.0.0.1', ...lanDevOrigins()],
    async rewrites() {
      return [
        {
          source: '/wow-source/:path*',
          destination: `http://127.0.0.1:${apiPort}/wow-source/:path*`,
        },
        {
          source: '/api/:path*',
          destination: `http://127.0.0.1:${apiPort}/api/:path*`,
        },
      ];
    },
  } : { output: 'export' }),
  reactStrictMode: false,
  eslint: {
    ignoreDuringBuilds: true,
  },
  typescript: {
    ignoreBuildErrors: true,
  },
  images: {
    unoptimized: true,
  },
  turbopack: {
    root: fileURLToPath(new URL('.', import.meta.url)),
  },
};

export default nextConfig;
