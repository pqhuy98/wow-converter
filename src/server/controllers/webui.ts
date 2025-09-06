import express from 'express';
import expressHttpProxy from 'express-http-proxy';
import fsExtra from 'fs-extra';
import type { Server as HttpServer } from 'http';
import httpProxy from 'http-proxy';
import path from 'path';

import { isDev } from '../config';

export function ControllerWebUi(app: express.Application, uiDir: string) {
  if (isDev) {
    // Proxy to nextjs dev server for hot reload
    devProxyServer(app);
    return true;
  }

  if (!fsExtra.existsSync(uiDir)) {
    return false;
  }

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

  return true;
}

let devWsProxy: ReturnType<typeof httpProxy.createProxyServer> | null = null;

function devProxyServer(app: express.Application) {
  const target = 'http://localhost:3000';

  // HTTP proxy for normal requests
  app.use('/', expressHttpProxy(target, {
    // Align headers with Next.js dev server expectations
    proxyReqOptDecorator: (proxyReqOpts) => {
      const headers = proxyReqOpts.headers ?? {};
      headers.origin = target;
      headers.host = 'localhost:3000';
      proxyReqOpts.headers = headers;
      return proxyReqOpts;
    },
  }));

  // WS proxy for HMR and dev sockets
  devWsProxy = httpProxy.createProxyServer({
    target,
    ws: true,
    changeOrigin: true,
    secure: false,
  });

  // Ensure Origin header matches target so Next dev server accepts the WS upgrade
  devWsProxy.on('proxyReqWs', (proxyReq) => {
    try {
      proxyReq.setHeader('origin', target);
      proxyReq.setHeader('host', 'localhost:3000');
    } catch (_e) {
      // noop
    }
  });

  // Prevent crashes on parse errors; log and ignore
  devWsProxy.on('error', (err) => {
    console.error('Dev WS proxy error:', err?.message || err);
  });

  // Send first request to trigger Next.js compilation
  fetch(target).catch(() => {});
}

export function attachDevWebsocketProxy(server: HttpServer) {
  if (!devWsProxy) return;
  server.on('upgrade', (req, socket, head) => {
    devWsProxy!.ws(req, socket, head);
  });
}
