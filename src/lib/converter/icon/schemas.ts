import { z } from 'zod';

// Zod schemas for icon types
export const IconSizeSchema = z.enum(['64x64', '128x128', '256x256', 'original']);
export const IconStyleSchema = z.enum(['classic-sd', 'reforged-hd', 'classic-hd-2.0']);
export const IconFrameSchema = z.enum(['btn', 'disbtn', 'pas', 'dispas', 'atc', 'disatc', 'att', 'upg', 'ssh', 'ssp', 'none']);

export const IconExtrasSchema = z.object({
  crop: z.boolean().default(false),
});

export const IconResizeModeSchema = z.enum(['normal', 'ai']).default('normal');

export const IconConversionOptionsSchema = z.object({
  size: IconSizeSchema,
  style: IconStyleSchema.default('classic-hd-2.0'),
  frame: IconFrameSchema.default('none'),
  extras: IconExtrasSchema.optional(),
  resizeMode: IconResizeModeSchema.optional(),
});

// Schema for POST body (extras as object, not string)
export const IconOptionsSchema = IconConversionOptionsSchema;

// TypeScript types inferred from Zod schemas
export type IconSize = z.infer<typeof IconSizeSchema>;
export type IconStyle = z.infer<typeof IconStyleSchema>;
export type IconFrame = z.infer<typeof IconFrameSchema>;
export type IconExtras = z.infer<typeof IconExtrasSchema>;
export type IconResizeMode = z.infer<typeof IconResizeModeSchema>;
export type IconConversionOptions = z.infer<typeof IconConversionOptionsSchema>;

// Type representing options after Zod defaults are applied (size, style, frame, resizeMode are required)
export type MergedIconConversionOptions = Required<Omit<IconConversionOptions, 'extras'>> & Partial<Pick<IconConversionOptions, 'extras'>>;
