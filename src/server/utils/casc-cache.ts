import crypto from 'crypto';
import type { Request, Response } from 'express';
import path from 'path';

import { isDev, isSharedHosting } from '@/server/config';

export function cascListCacheMaxAge(): number {
  if (isDev) return 60;
  if (isSharedHosting) return 3600;
  return 3600;
}

export function cascMinimapCacheMaxAge(): number {
  if (isDev) return 300;
  return 86400;
}

export function etagFromParts(...parts: string[]): string {
  return `"${crypto.createHash('md5').update(parts.join('|')).digest('hex')}"`;
}

/** Long cache when ?build= matches active CASC; otherwise revalidate via ETag. */
export function applyCascBuildCache(
  res: Response,
  req: Request,
  activeBuild: string,
  etag: string,
  minimap = false,
): void {
  res.setHeader('ETag', etag);
  const reqBuild = typeof req.query.build === 'string' ? req.query.build : '';
  if (reqBuild && reqBuild === activeBuild) {
    const maxAge = minimap ? cascMinimapCacheMaxAge() : cascListCacheMaxAge();
    res.setHeader('Cache-Control', `public, max-age=${maxAge}`);
    return;
  }
  res.setHeader('Cache-Control', 'private, no-cache');
}

export function matchNotModified(req: Request, etag: string): boolean {
  return req.headers['if-none-match'] === etag;
}

export function writeNotModified(res: Response, etag: string): void {
  res.setHeader('ETag', etag);
  res.status(304).end();
}

/** Append active CASC buildKey so browser cache keys rotate on version switch. */
export function withCascBuild(url: string, buildKey?: string): string {
  if (!buildKey) return url;
  const join = url.includes('?') ? '&' : '?';
  return `${url}${join}build=${encodeURIComponent(buildKey)}`;
}

/** Disk path for a decoded minimap tile scoped to the active CASC build. */
export function minimapPngPath(
  assetDir: string,
  buildKey: string,
  mapDir: string,
  xs: string,
  ys: string,
): string {
  const parts = ['world', 'minimaps'];
  if (buildKey) parts.push('_casc', buildKey);
  parts.push(mapDir, `map${xs}_${ys}.png`);
  return path.join(assetDir, ...parts);
}
