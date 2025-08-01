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

async function main() {
  await ControllerExportCharacter(app);
  ControllerDownload(app);

  // serve the static UI
  const uiDir = path.join('webui', 'out');
  let isWithUI = false;
  if (fsExtra.existsSync(uiDir)) {
    app.use(express.static(uiDir));

    // Handle client-side routing - serve the correct HTML file based on the path
    app.use((req, res, next) => {
      // Skip static file requests (they should be handled by express.static)
      if (req.path.includes('.')) {
        return next();
      }

      // Determine which HTML file to serve based on the path
      let htmlFile = 'index.html';

      // If the path is not root, try to serve the corresponding HTML file
      if (req.path !== '/') {
        const requestedFile = `${req.path.slice(1)}.html`; // Remove leading slash and add .html
        const filePath = path.resolve(uiDir, requestedFile);
        if (fsExtra.existsSync(filePath)) {
          htmlFile = requestedFile;
          // We are not afraid of path traversal here because Express JS already sanitizes the path
          // Tried http://127.0.0.1:3001/../../password.html, the path is sanitized to /password.html
        }
      }
      return res.sendFile(htmlFile, { root: path.resolve(uiDir) });
    });

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
}

main().catch(console.error);
