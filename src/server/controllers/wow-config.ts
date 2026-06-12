import type { Router } from 'express';

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
import { pickNativeFolder } from '../utils/pick-folder';

export function ControllerWowConfig(router: Router): void {
  router.get('/wow-config/status', async (_req, res, next) => {
    try {
      res.json(await getWowConfigStatus());
    } catch (e) {
      next(e);
    }
  });

  router.post('/wow-config/pick-local-folder', (_req, res) => {
    if (isSharedHosting) {
      res.status(403).json({ error: 'Folder picker is not available in shared hosting mode' });
      return;
    }
    try {
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
      res.status(500).json({ error: (e as Error).message });
    }
  });

  router.post('/wow-config/reset', async (_req, res) => {
    try {
      await resetWowConfig();
      res.json(await getWowConfigStatus());
    } catch (e) {
      res.status(400).json({ error: (e as Error).message });
    }
  });

  router.post('/wow-config/discover-local', async (req, res) => {
    try {
      const installDirectory = req.body?.installDirectory;
      if (typeof installDirectory !== 'string' || !installDirectory.trim()) {
        res.status(400).json({ error: 'installDirectory is required' });
        return;
      }
      const builds = await discoverLocalBuilds(normalizeInstallDirectory(installDirectory));
      res.json({ builds });
    } catch (e) {
      res.status(400).json({ error: (e as Error).message });
    }
  });

  router.post('/wow-config/discover-remote', async (req, res) => {
    try {
      const regionTag = req.body?.regionTag;
      if (typeof regionTag !== 'string' || !regionTag.trim()) {
        res.status(400).json({ error: 'regionTag is required' });
        return;
      }
      const builds = await discoverRemoteBuilds(regionTag.trim());
      res.json({ builds });
    } catch (e) {
      res.status(400).json({ error: (e as Error).message });
    }
  });

  router.post('/wow-config/apply', async (req, res) => {
    try {
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
      res.status(400).json({ error: (e as Error).message });
    }
  });
}
