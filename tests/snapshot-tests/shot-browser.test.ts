import { expect, test } from 'bun:test';

import { ShotBrowser } from '../../.cursor/skills/shot-export-wow-converter/shot-export';
import { decodePng } from './model/visual-png.helper';

test('isolates browsers and captures rewritten assets without cache or development overlays', async () => {
  let color = 'red';
  const server = Bun.serve({
    hostname: '127.0.0.1',
    port: 0,
    fetch(req) {
      if (new URL(req.url).pathname === '/texture.svg') {
        return new Response(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><rect width="20" height="20" fill="${color}"/></svg>`, {
          headers: { 'content-type': 'image/svg+xml', 'cache-control': 'public, max-age=3600' },
        });
      }
      return new Response(`<!doctype html><html><head><style>body{margin:0}nextjs-portal{position:fixed;inset:0;background:lime}</style></head><body><canvas width="20" height="20"></canvas><nextjs-portal></nextjs-portal><script>
        window.__shotView = view => view;
        const image = new Image();
        image.onload = () => {
          document.querySelector('canvas').getContext('2d').drawImage(image, 0, 0);
          document.documentElement.dataset.viewerReady = '1';
          document.documentElement.dataset.viewerSequence = 'Stand';
        };
        image.src = '/texture.svg';
      </script></body></html>`, { headers: { 'content-type': 'text/html' } });
    },
  });
  const browsers: ShotBrowser[] = [];
  try {
    browsers.push(await ShotBrowser.open(640, 400));
    browsers.push(await ShotBrowser.open(640, 400));
    for (const [nextColor, pixel] of [['red', [255, 0, 0, 255]], ['blue', [0, 0, 255, 255]]] as const) {
      color = nextColor;
      for (const browser of browsers) {
        const captured = await browser.capture({
          base: `http://127.0.0.1:${server.port}`, model: 'same.mdx', seq: 'Stand', width: 640, height: 400, views: ['front'],
        });
        const png = captured.views.get('front');
        if (!png) throw new Error('missing front capture');
        expect(Array.from(decodePng(png).data.slice(0, 4))).toEqual([...pixel]);
      }
    }
  } finally {
    await Promise.all(browsers.map((browser) => browser.close()));
    await server.stop(true);
  }
}, 60_000);
