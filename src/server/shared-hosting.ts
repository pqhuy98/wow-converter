import { isSharedHosting } from './config';

export const DESKTOP_ONLY_MESSAGE = 'This feature is only available in the desktop app, not on shared hosting.';

export function assertDesktopOnly(): void {
  if (isSharedHosting) {
    throw new Error(DESKTOP_ONLY_MESSAGE);
  }
}

export function desktopOnlyStatus(error: Error): number {
  return error.message === DESKTOP_ONLY_MESSAGE ? 403 : 400;
}
