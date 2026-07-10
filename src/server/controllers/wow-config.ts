import type { Router } from 'express';

import { clearProjectCacheDir, getProjectCacheDirSize } from '@/lib/wow/clear-project-cache';
import { normalizeInstallDirectory } from '@/lib/wow/normalize-install-directory';
import {
  applyWowConfig,
  discoverLocalBuilds,
  discoverRemoteBuilds,
  getWowConfigStatus,
  resetWowConfig,
} from '@/lib/wow/wow-config-service';
import type { WowConfig } from '@/lib/wow/wow-config-state';

import { isSharedHosting } from '../config';
import { assertDesktopOnly, desktopOnlyStatus } from '../shared-hosting';
import { pickNativeFolder } from '../utils/pick-folder';

const WOW_CONFIG_SHARED_HOSTING_LOCKED = 'WoW installation cannot be changed in shared hosting mode.';

function assertWowConfigMutable(): void {
  if (isSharedHosting) {
    throw new Error(WOW_CONFIG_SHARED_HOSTING_LOCKED);
  }
}

function wowConfigErrorStatus(error: Error): number {
  return error.message === WOW_CONFIG_SHARED_HOSTING_LOCKED ? 403 : 400;
}

export function ControllerWowConfig(router: Router): void {
  router.get('/wow-config/status', async (_req, res, next) => {
    try {
      res.setHeader('Cache-Control', 'no-store');
      res.json(await getWowConfigStatus());
    } catch (e) {
      next(e);
    }
  });

  router.post('/wow-config/pick-local-folder', (_req, res) => {
    try {
      assertWowConfigMutable();
      const startDirectory = typeof _req.body?.installDirectory === 'string'
        ? normalizeInstallDirectory(_req.body.installDirectory)
        : undefined;
      const installDirectory = pickNativeFolder(
        'Select World of Warcraft install folder',
        startDirectory,
      );
      if (!installDirectory) {
        res.json({ cancelled: true });
        return;
      }
      res.json({ installDirectory });
    } catch (e) {
      const err = e as Error;
      res.status(wowConfigErrorStatus(err)).json({ error: err.message });
    }
  });

  router.post('/wow-config/reset', async (_req, res) => {
    try {
      assertWowConfigMutable();
      await resetWowConfig();
      res.json(await getWowConfigStatus());
    } catch (e) {
      const err = e as Error;
      res.status(wowConfigErrorStatus(err)).json({ error: err.message });
    }
  });

  router.get('/wow-config/cache-size', async (_req, res, next) => {
    try {
      assertDesktopOnly();
      const bytes = await getProjectCacheDirSize();
      res.json({ bytes });
    } catch (e) {
      const err = e as Error;
      if (desktopOnlyStatus(err) === 403) {
        res.status(403).json({ error: err.message });
        return;
      }
      next(e);
    }
  });

  router.post('/wow-config/clear-cache', async (_req, res) => {
    try {
      assertDesktopOnly();
      await clearProjectCacheDir();
      res.json(await getWowConfigStatus());
    } catch (e) {
      const err = e as Error;
      res.status(desktopOnlyStatus(err)).json({ error: err.message });
    }
  });

  router.post('/wow-config/discover-local', async (req, res) => {
    try {
      assertWowConfigMutable();
      const installDirectory = req.body?.installDirectory;
      if (typeof installDirectory !== 'string' || !installDirectory.trim()) {
        res.status(400).json({ error: 'installDirectory is required' });
        return;
      }
      const builds = await discoverLocalBuilds(normalizeInstallDirectory(installDirectory));
      res.json({ builds });
    } catch (e) {
      const err = e as Error;
      res.status(wowConfigErrorStatus(err)).json({ error: err.message });
    }
  });

  router.post('/wow-config/discover-remote', async (req, res) => {
    try {
      assertWowConfigMutable();
      const regionTag = req.body?.regionTag;
      if (typeof regionTag !== 'string' || !regionTag.trim()) {
        res.status(400).json({ error: 'regionTag is required' });
        return;
      }
      const builds = await discoverRemoteBuilds(regionTag.trim());
      res.json({ builds });
    } catch (e) {
      const err = e as Error;
      res.status(wowConfigErrorStatus(err)).json({ error: err.message });
    }
  });

  router.post('/wow-config/apply', async (req, res) => {
    try {
      assertWowConfigMutable();
      const {
        mode, installDirectory, regionTag, product,
      } = req.body ?? {};
      if (mode !== 'local' && mode !== 'remote') {
        res.status(400).json({ error: 'mode must be "local" or "remote"' });
        return;
      }
      if (typeof product !== 'string' || !product.trim()) {
        res.status(400).json({ error: 'product is required' });
        return;
      }

      let config: WowConfig;
      if (mode === 'local') {
        if (typeof installDirectory !== 'string' || !installDirectory.trim()) {
          res.status(400).json({ error: 'installDirectory is required for local mode' });
          return;
        }
        config = {
          mode: 'local',
          installDirectory: normalizeInstallDirectory(installDirectory),
          product: product.trim(),
        };
      } else {
        if (typeof regionTag !== 'string' || !regionTag.trim()) {
          res.status(400).json({ error: 'regionTag is required for remote mode' });
          return;
        }
        config = {
          mode: 'remote',
          regionTag: regionTag.trim(),
          product: product.trim(),
        };
      }

      const cascInfo = await applyWowConfig(config);
      const status = await getWowConfigStatus();
      res.json({ cascInfo, status });
    } catch (e) {
      const err = e as Error;
      res.status(wowConfigErrorStatus(err)).json({ error: err.message });
    }
  });
}
