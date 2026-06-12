// Express API port — do not use PORT (Next.js sets that to 3000 in dev).
const apiPort = process.env.API_PORT || process.env.EXPRESS_PORT || '3001';
const isDev = process.env.NODE_ENV === 'development';

/** @type {import('next').NextConfig} */
const nextConfig = {
  allowedDevOrigins: ['localhost:3001', '127.0.0.1:3001'],
  // Static export for production builds only — dev needs rewrites to proxy /api to Express.
  ...(isDev ? {} : { output: 'export' }),
  async rewrites() {
    if (!isDev) return [];
    return [
      {
        source: '/api/:path*',
        destination: `http://127.0.0.1:${apiPort}/api/:path*`,
      },
    ];
  },
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
    root: './',
  },
};

export default nextConfig;
