import express from 'express';

import { wowDataClient } from '@/lib/wow-data-client/wow-data-client';

import { isDev, isSharedHosting } from '../config';
import { getCeConfig } from './export-character';

export function ControllerGetConfig(router: express.Router) {
  router.get('/get-config', (req, res) => {
    res.json({
      exportAssetDir: getCeConfig().exportAssetDir,
      isSharedHosting,
      isDev,
      isClassic: wowDataClient.isClassic(),
    });
  });
}
