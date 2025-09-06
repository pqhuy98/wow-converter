export const formatTimestamp = (timestamp: number, showAbsolute: boolean = false): string => {
  if (showAbsolute) {
    const date = new Date(timestamp);
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    const seconds = String(date.getSeconds()).padStart(2, '0');
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
  }

  const now = Date.now();
  const diff = now - timestamp;
  const minutes = Math.floor(diff / (1000 * 60));
  const hours = Math.floor(diff / (1000 * 60 * 60));
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));

  if (days > 0) {
    return `${days} day${days > 1 ? 's' : ''} ago`;
  } if (hours > 0) {
    return `${hours} hour${hours > 1 ? 's' : ''} ago`;
  } if (minutes > 0) {
    return `${minutes} min${minutes > 1 ? 's' : ''} ago`;
  }
  return 'Just now';
};

export function formatDurationBetween(startTimeMs?: number, endTimeMs?: number): string | null {
  if (!startTimeMs || !endTimeMs) return null;

  const duration = endTimeMs - startTimeMs;
  const seconds = Math.floor(duration / 1000);
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;

  if (minutes > 0) {
    return `${minutes}m ${remainingSeconds}s`;
  }
  return `${seconds}s`;
}

export function formatFileSize(size: number): string {
  const MAX_B = 1024;
  const MAX_KB = MAX_B * MAX_B;
  const MAX_MB = MAX_KB * MAX_B;

  if (size < MAX_B) {
    return `${size}B`;
  }
  if (size < MAX_KB) {
    return `${(size / MAX_B).toFixed(2)}KB`;
  }
  if (size < MAX_MB) {
    return `${(size / MAX_KB).toFixed(2)}MB`;
  }
  return `${(size / MAX_MB).toFixed(2)}GB`;
}
