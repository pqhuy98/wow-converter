const LOCALHOST_ORIGIN = /^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?$/;

export function isAllowedCorsOrigin(origin: string): boolean {
  return LOCALHOST_ORIGIN.test(origin);
}
