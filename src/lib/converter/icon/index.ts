// Main entry point - export IconExporter as the primary export
export { IconExporter } from './icon-exporter';
export type { IconExportItem } from './icon-exporter';

// Re-export commonly used types and schemas
export type {
  IconConversionOptions,
  IconExtras,
  IconFrame,
  IconResizeMode,
  IconSize,
  IconStyle,
} from './schemas';
export {
  IconConversionOptionsSchema,
  IconExtrasSchema,
  IconFrameSchema,
  IconOptionsSchema,
  IconResizeModeSchema,
  IconSizeSchema,
  IconStyleSchema,
} from './schemas';

// Re-export utilities
export { getCustomFrameData, mergeIconOptions } from './utils';
export { getWc3Path } from './wc3.utils';

// Re-export constants
export {
  FRAME_FILE_MAP,
  HD_DESATURATION_FRAMES,
  SIZE_MAPPING,
  STYLE_FOLDER_MAP,
} from './constants';
