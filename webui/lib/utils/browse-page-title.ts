const SITE_TITLE = 'Huy\'s wow-converter';

export function getBrowsePageTitle(search?: string | null, selectedPath?: string | null): string {
  if (selectedPath) {
    return selectedPath.split(/[/\\]/).pop() ?? 'Browse Models';
  }

  const trimmed = search?.trim();
  if (trimmed) return trimmed;

  return 'Browse Models';
}

export function formatDocumentTitle(pageTitle: string): string {
  return `${pageTitle} | ${SITE_TITLE}`;
}
