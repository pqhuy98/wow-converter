const baseURL = (process.env.WOW_CONVERTER_URL ?? 'http://127.0.0.1:3001').replace(/\/$/, '');

export function converterURL(): string {
  return baseURL;
}

export async function api(path: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(`${baseURL}${path}`, init);
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`converter is not running at ${baseURL}: ${message}`);
  }
}

export async function ensureCascLoaded(): Promise<void> {
  const deadline = Date.now() + 3 * 60_000;
  let last = '';
  while (Date.now() < deadline) {
    const response = await api('/api/wow-config/status');
    const body: unknown = await response.json();
    if (isRecord(body) && body.cascLoaded === true && body.cascLoading === false && body.wowDataServerReachable === true) {
      return;
    }
    last = JSON.stringify(body).slice(0, 300);
    await Bun.sleep(1_000);
  }
  throw new Error(`converter at ${baseURL} did not finish loading CASC: ${last}`);
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export async function readBody(response: Response): Promise<unknown> {
  const text = await response.text();
  if (response.status >= 400) {
    throw new Error(`${response.status} ${response.url}: ${text.slice(0, 400)}`);
  }
  if (text.trim() === '') return null;
  return JSON.parse(text) as unknown;
}

export function jsonInit(body: unknown): RequestInit {
  return {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  };
}
