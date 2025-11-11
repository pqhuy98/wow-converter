import type { IconFrame } from '@/lib/models/icon-export.model';
import { getWc3Path } from '@/lib/utils/wc3.utils';

/**
 * Extract base name from texture path (filename without extension)
 * @param texturePath - Full texture path (e.g., "interface/icons/inv_belt_18.blp")
 * @returns Base name without extension (e.g., "inv_belt_18")
 */
export function extractBaseName(texturePath: string): string {
  const filename = texturePath.split('/').pop() ?? texturePath;
  return filename.replace(/\.(blp|png|jpg|jpeg)$/i, '');
}

/**
 * Generate default output name for an icon based on texture path and frame
 * - For 'none' frame (raw): returns base name without underscore
 * - For other frames: returns base name with underscore prefix
 * @param texturePath - Full texture path
 * @param frame - Icon frame type
 * @returns Default output name
 */
export function getDefaultOutputName(texturePath: string, frame: IconFrame): string {
  const baseName = extractBaseName(texturePath);
  return frame === 'none' ? baseName : `_${baseName}`;
}

/**
 * Format icon name for display based on output name and frame
 * - For 'none' frame (raw): displays full outputName as-is to preserve path structure
 * - For other frames: generates Wc3 path and returns basename (filename without extension)
 * @param texturePath - Full texture path (used for fallback)
 * @param frame - Icon frame type
 * @param outputName - Output name (may include path separators for raw frame)
 * @returns Formatted display name
 */
export function formatIconName(texturePath: string, frame: IconFrame, outputName: string): string {
  // For 'none' frame (raw), display the full outputName as-is to preserve path structure
  if (frame === 'none') {
    return outputName;
  }

  // For other frames, generate Wc3 path and return its basename (filename without extension)
  const wc3Path = getWc3Path(`${outputName}.blp`, frame);
  const filename = wc3Path.split('/').pop() ?? wc3Path;
  return filename.replace(/\.blp$/i, '');
}

/**
 * Generate Wc3 path for tooltip display
 * Matches the logic used when adding icons to cart
 * @param texturePath - Full texture path
 * @param frame - Icon frame type
 * @returns Wc3 output path
 */
export function getWc3PathForTooltip(texturePath: string, frame: IconFrame): string {
  const outputName = getDefaultOutputName(texturePath, frame);
  return getWc3Path(`${outputName}.blp`, frame);
}
