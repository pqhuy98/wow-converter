/**
 * CDN host resolution with intelligent pre-caching, ported from wow.export
 * (src/js/casc/cdn-resolver.js).
 */
import util from 'util';

import { write } from '@/lib/wow/log';

import { constants } from '../../formats/constants';
import { get, ping } from '../../formats/generics';
import { parseVersionConfig, VersionConfigEntry } from './version-config';

interface RankedHost {
  host: string;
  ping: number;
}

interface ResolutionCacheEntry {
  promise: Promise<RankedHost[]> | null;
  bestHost: RankedHost | null;
  rankedHosts?: RankedHost[] | null;
}

class CDNResolver {
  /** Map of cacheKey -> { promise, bestHost }. cacheKey = region + '|' + hosts. */
  private resolutionCache = new Map<string, ResolutionCacheEntry>();

  /** Track hosts that have failed to respond properly (e.g., censored responses). */
  private failedHosts = new Set<string>();

  /** Start pre-resolution for a region if not already started. */
  startPreResolution(region: string, product = 'wow'): void {
    write('Starting CDN pre-resolution for region: %s', region);
    void this._resolveRegionProduct(region, product);
  }

  /** Get the best host for a region with specific server config. */
  async getBestHost(region: string, serverConfig: VersionConfigEntry): Promise<string> {
    const cacheKey = this._getCacheKey(region, serverConfig.Hosts);
    const cached = this.resolutionCache.get(cacheKey);

    if (cached?.bestHost) {
      write('Using cached CDN host for %s: %s', region, cached.bestHost.host);
      return `${cached.bestHost.host}${serverConfig.Path}/`;
    }

    if (cached?.promise) {
      write('Waiting for CDN resolution for %s', region);
      const result = await cached.promise;
      return `${result[0].host}${serverConfig.Path}/`;
    }

    write('Resolving CDN hosts for %s: %s', region, serverConfig.Hosts);
    const promise = this._resolveHosts(region, serverConfig);

    this.resolutionCache.set(cacheKey, { promise, bestHost: null });

    const rankedHosts = await promise;
    this.resolutionCache.set(cacheKey, {
      promise: null,
      bestHost: rankedHosts[0],
      rankedHosts,
    });

    return `${rankedHosts[0].host}${serverConfig.Path}/`;
  }

  /**
   * Get all available hosts for a region ranked by ping speed.
   * Excludes hosts that have previously failed.
   */
  async getRankedHosts(region: string, serverConfig: VersionConfigEntry): Promise<string[]> {
    const cacheKey = this._getCacheKey(region, serverConfig.Hosts);
    const cached = this.resolutionCache.get(cacheKey);

    if (cached?.rankedHosts) {
      write('Using cached ranked CDN hosts for %s', region);
      return cached.rankedHosts.map((h) => `${h.host}${serverConfig.Path}/`);
    }

    if (cached?.promise) {
      write('Waiting for CDN resolution for %s', region);
      await cached.promise;
      const updated = this.resolutionCache.get(cacheKey)!;
      return updated.rankedHosts!.map((h) => `${h.host}${serverConfig.Path}/`);
    }

    write('Resolving CDN hosts for %s: %s', region, serverConfig.Hosts);
    const promise = this._resolveHosts(region, serverConfig);

    this.resolutionCache.set(cacheKey, { promise, bestHost: null, rankedHosts: null });

    const rankedHosts = await promise;
    this.resolutionCache.set(cacheKey, {
      promise: null,
      bestHost: rankedHosts[0],
      rankedHosts,
    });

    return rankedHosts.map((h) => `${h.host}${serverConfig.Path}/`);
  }

  /** Mark a host as failed (e.g., due to censorship or invalid responses). */
  markHostFailed(host: string): void {
    write('Marking CDN host as failed: %s', host);
    this.failedHosts.add(host);
  }

  private _getCacheKey(region: string, hosts: string): string {
    return `${region}|${hosts}`;
  }

  private async _resolveRegionProduct(region: string, product: string): Promise<void> {
    try {
      const host = util.format(constants.PATCH.HOST, region);
      const url = host + product + constants.PATCH.SERVER_CONFIG;
      const res = await get(url);

      if (!res.ok) throw new Error(util.format('HTTP %d from server config endpoint: %s', res.status, url));

      const serverConfigs = parseVersionConfig(await res.text());
      const serverConfig = serverConfigs.find((e) => e.Name === region);

      if (!serverConfig) throw new Error(`CDN config does not contain entry for region ${region}`);

      // Use getBestHost to resolve and cache
      await this.getBestHost(region, serverConfig);
    } catch (error) {
      write('Failed to pre-resolve CDN hosts for region %s: %s', region, (error as Error).message);
    }
  }

  /** Ping all hosts in server config and rank them by speed. */
  private async _resolveHosts(region: string, serverConfig: VersionConfigEntry): Promise<RankedHost[]> {
    write('Resolving best host for %s: %s', region, serverConfig.Hosts);

    const hosts = serverConfig.Hosts.split(' ').map((e) => `https://${e}/`);
    const validHosts: RankedHost[] = [];
    const hostPings: Promise<void>[] = [];

    for (const host of hosts) {
      if (this.failedHosts.has(host)) {
        write('Skipping previously failed host: %s', host);
        continue;
      }

      hostPings.push(ping(host).then((pingMs) => {
        write('Host %s resolved with %dms ping', host, pingMs);
        validHosts.push({ host, ping: pingMs });
      }).catch((e) => {
        write('Host %s failed to resolve a ping: %s', host, e);
      }));
    }

    await Promise.allSettled(hostPings);

    if (validHosts.length === 0) throw new Error('Unable to resolve any CDN hosts (all failed or blocked).');

    validHosts.sort((a, b) => a.ping - b.ping);

    write('%s resolved as the fastest host with a ping of %dms', validHosts[0].host, validHosts[0].ping);
    return validHosts;
  }
}

export const cdnResolver = new CDNResolver();

export default cdnResolver;
