import 'dotenv/config';

import chalk from 'chalk';
import cors from 'cors';
import express from 'express';
import fsExtra from 'fs-extra';
import path from 'path';

import { printLogo } from '../lib/logo';
import { ControllerDownload } from './controllers/download';
import { ControllerExportCharacter } from './controllers/export-character';

printLogo();
const app = express();
app.use(cors({
  origin: process.env.UI_DOMAIN ?? '*',
  methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
}));
app.use(express.json());

await ControllerExportCharacter(app);
ControllerDownload(app);

// serve the static UI
const uiDir = path.join('webui', 'out');
let isWithUI = false;
if (fsExtra.existsSync(uiDir)) {
  app.use(express.static(uiDir));
  app.get('/', (_, res) => res.sendFile(path.join(uiDir, 'index.html')));
  isWithUI = true;
}

// Error-handling middleware (must be **after** all routes)
app.use((err: Error, _req: express.Request, res: express.Response, _next: express.NextFunction) => {
  console.error(err);
  res.status(500).json({ error: err.message ?? err });
});

const port = process.env.PORT ? parseInt(process.env.PORT, 10) : 3001;
app.listen(port, () => {
  if (isWithUI) {
    console.log(`Serving UI web interface at ${chalk.blue(`http://127.0.0.1:${port}/`)}`);
  } else {
    console.log(`Serving only REST API at ${chalk.blue(`http://127.0.0.1:${port}/`)}`);
  }
});
