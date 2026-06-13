import http from 'http';
import { URL } from 'url';

import { getDataServerHttpOrigin, getDataServerSocketPath } from './transport';

export interface DataServerHttpResult {
  ok: boolean;
  status: number;
  json: Record<string, unknown>;
  body: Buffer;
}

function parseOrigin(origin: string): { hostname: string; port: number } {
  const url = new URL(origin);
  return {
    hostname: url.hostname || '127.0.0.1',
    port: url.port ? Number(url.port) : 80,
  };
}

export async function dataServerHttpRequest(
  method: 'GET' | 'POST',
  requestPath: string,
  body?: unknown,
): Promise<DataServerHttpResult> {
  const socketPath = getDataServerSocketPath();
  const { hostname, port } = parseOrigin(getDataServerHttpOrigin());
  const payload = body === undefined ? undefined : JSON.stringify(body);

  return new Promise((resolve) => {
    const req = http.request({
      method,
      path: requestPath,
      ...(socketPath
        ? { socketPath }
        : { hostname, port }),
      headers: payload
        ? {
          'Content-Type': 'application/json',
          'Content-Length': Buffer.byteLength(payload),
        }
        : undefined,
    }, (res) => {
      const chunks: Buffer[] = [];
      res.on('data', (chunk: Buffer) => chunks.push(chunk));
      res.on('end', () => {
        const buf = Buffer.concat(chunks);
        let json: Record<string, unknown> = {};
        if (buf.length > 0) {
          try {
            json = JSON.parse(buf.toString('utf-8')) as Record<string, unknown>;
          } catch {
            json = { id: 'ERR_INVALID_JSON' };
          }
        }
        const status = res.statusCode ?? 0;
        resolve({
          ok: status >= 200 && status < 300,
          status,
          json,
          body: buf,
        });
      });
    });

    req.on('error', (err) => {
      resolve({
        ok: false,
        status: 0,
        json: { id: 'ERR_UNREACHABLE', message: err.message },
        body: Buffer.alloc(0),
      });
    });

    if (payload) req.write(payload);
    req.end();
  });
}

export async function dataServerGetJson(path: string): Promise<DataServerHttpResult> {
  return dataServerHttpRequest('GET', path);
}

export async function dataServerPostJson(path: string, body: unknown): Promise<DataServerHttpResult> {
  return dataServerHttpRequest('POST', path, body);
}
