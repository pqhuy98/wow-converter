/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  allowedDevOrigins: ['localhost:3001', "127.0.0.1:3001"],
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
  /**
   * Stub Node.js-specific modules (e.g. 'fs') so that browser bundles do not
   * attempt to include them. This fixes build-time errors coming from
   * dependencies that conditionally require these modules when they detect a
   * Node environment – for example fengari inside mdx-m3-viewer.
   */
  webpack: (config, { isServer }) => {
    if (!isServer) {
      config.resolve.fallback = {
        ...config.resolve.fallback,
        fs: false,
        path: false,
        os: false,
        crypto: false,
        stream: false,
      };
    }
    return config;
  },
}

export default nextConfig
