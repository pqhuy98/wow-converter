import { usesSocketTransport } from '@/lib/wow-data-server/transport';

export const DATA_SERVER_TOKEN_HEADER = 'X-Wow-Data-Token';

export function configuredDataServerToken(): string {
  return process.env.WOW_DATA_SERVER_TOKEN ?? '';
}

export function dataServerAuthRequired(): boolean {
  if (usesSocketTransport()) return false;
  return configuredDataServerToken().length > 0;
}

export function authorizeDataServerRequest(req: { headers: Record<string, string | string[] | undefined> }): boolean {
  if (!dataServerAuthRequired()) return true;
  const want = configuredDataServerToken();
  const got = req.headers[DATA_SERVER_TOKEN_HEADER.toLowerCase()];
  const value = Array.isArray(got) ? got[0] : got;
  return typeof value === 'string' && value.length > 0 && value === want;
}
