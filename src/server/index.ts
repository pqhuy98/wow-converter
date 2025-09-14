import 'dotenv/config';

import chalk from 'chalk';
import compression from 'compression';
import cors from 'cors';
import express from 'express';
import helmet from 'helmet';
import path from 'path';

import { printLogo } from '../lib/logo';
import { isDev, isSharedHosting } from './config';
import { ControllerBrowse } from './controllers/browse';
import { ControllerDownload } from './controllers/download';
import { ControllerExportCharacter } from './controllers/export-character';
import { ControllerGetConfig } from './controllers/get-config';
import { attachDevWebsocketProxy, ControllerWebUi } from './controllers/webui';

printLogo();
const app = express();
app.use(compression());
app.use(helmet({
  contentSecurityPolicy: {
    useDefaults: true,
    directives: {
      // defaultSrc: ["'self'"],
      scriptSrc: ["'self'", "'unsafe-inline'", "'unsafe-eval'"],
      // styleSrc: ["'self'", "'unsafe-inline'"],
    },
  },
}));

// Only allow CORS for personal mode or local development
if (!isSharedHosting || isDev) {
  app.use(cors({
    origin: '*',
    methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
  }));
}

app.use(express.json());

const router = express.Router();
const uiRouter = express.Router();

async function main() {
  await ControllerExportCharacter(router);
  ControllerDownload(router);
  ControllerGetConfig(router);
  ControllerBrowse(router);

  // serve the static UI
  const uiDir = path.join('webui', 'out');
  const isWithUI = ControllerWebUi(uiRouter, uiDir);

  app.use('/api', router);
  if (isWithUI) {
    app.use('/', uiRouter);
  }

  // Error-handling middleware (must be **after** all routes)
  app.use((err: Error, _req: express.Request, res: express.Response, _next: express.NextFunction) => {
    console.error(err);
    res.status(500).json({ error: err.message ?? err });
  });

  const port = process.env.PORT ? parseInt(process.env.PORT, 10) : 3001;
  const server = app.listen(port, () => {
    if (isWithUI) {
      console.log(`Serving UI web interface at ${chalk.blue(`http://127.0.0.1:${port}/`)}`);
    } else {
      console.log(`Found no UI, serving only REST API at ${chalk.blue(`http://127.0.0.1:${port}/`)}`);
    }
  });

  if (isDev) {
    attachDevWebsocketProxy(server);
  }
}

main().catch(console.error);
