const allowedFetchHosts = new Set([
  'wowhead.com',
  'www.wowhead.com',
  'wow.zamimg.com',
  'nether.wowhead.com',
]);

function isAllowedFetchHost(host: string): boolean {
  const normalized = host.toLowerCase();
  if (allowedFetchHosts.has(normalized)) {
    return true;
  }
  return normalized.endsWith('.wowhead.com');
}

export function validateFetchUrl(raw: string): URL {
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error('Invalid URL');
  }
  if (url.protocol !== 'https:') {
    throw new Error('Unsupported URL scheme');
  }
  if (url.username || url.password) {
    throw new Error('Invalid URL');
  }
  if (!isAllowedFetchHost(url.hostname)) {
    throw new Error('URL host not allowed');
  }
  return url;
}

/** Headers that CloudFront/WAF often expect for document-style HTML fetches (matches real Chrome navigation). */
const defaultBrowserHeaders: Record<string, string> = {
  Accept:
    'text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7',
  'Accept-Language': 'en-US,en;q=0.9',
  'Cache-Control': 'no-cache',
  DNT: '1',
  Pragma: 'no-cache',
  'Sec-CH-UA': '"Chromium";v="146", "Not-A.Brand";v="24", "Google Chrome";v="146"',
  'Sec-CH-UA-Mobile': '?0',
  'Sec-CH-UA-Platform': '"Windows"',
  'Sec-Fetch-Dest': 'document',
  'Sec-Fetch-Mode': 'navigate',
  'Sec-Fetch-Site': 'none',
  'Sec-Fetch-User': '?1',
  'Upgrade-Insecure-Requests': '1',
  'User-Agent':
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36',
};

export function customFetch(url: string, options: RequestInit = {}): Promise<Response> {
  validateFetchUrl(url);
  return fetch(url, {
    ...options,
    headers: {
      ...options.headers,
      ...defaultBrowserHeaders,
    },
    redirect: 'follow',
  });
}
