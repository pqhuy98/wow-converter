// Main entry point - export IconExporter as the primary export
export { IconExporter } from './icon-exporter';
export type { IconExportItem } from './icon-exporter';

// Re-export commonly used types and schemas
export type {
  IconConversionOptions,
  IconExtras,
  IconFrame,
  IconSize,
  IconStyle,
  RequiredIconConversionOptions,
} from './schemas';
export {
  IconConversionOptionsSchema,
  IconExtrasSchema,
  IconFrameSchema,
  IconOptionsSchema,
  IconSizeSchema,
  IconStyleSchema,
} from './schemas';

// Re-export utilities
export { getCustomFrameData, mergeIconOptions, resolveEffectiveSize } from './utils';

// Re-export constants
export {
  DEFAULT_ICON_OPTIONS,
  FRAME_FILE_MAP,
  HD_DESATURATION_FRAMES,
  SIZE_MAPPING,
  STYLE_FOLDER_MAP,
} from './constants';
