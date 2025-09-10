import express from 'express';

import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { isDev, isSharedHosting } from '../config';
import { getCeConfig } from './export-character';

export function ControllerGetConfig(router: express.Router) {
  router.get('/get-config', (req, res) => {
    res.json({
      wowExportAssetDir: getCeConfig().wowExportAssetDir,
      isSharedHosting,
      isDev,
      isClassic: wowExportClient.isClassic(),
    });
  });
}
