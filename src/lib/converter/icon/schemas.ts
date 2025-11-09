import { z } from 'zod';

// Zod schemas for icon types
export const IconSizeSchema = z.enum(['64x64', '128x128', '256x256', 'original']);
export const IconStyleSchema = z.enum(['classic-sd', 'reforged-hd', 'classic-hd-2.0']);
export const IconFrameSchema = z.enum(['btn', 'disbtn', 'pas', 'dispas', 'atc', 'disatc', 'att', 'upg', 'ssh', 'ssp', 'none']);

export const IconExtrasSchema = z.object({
  crop: z.boolean().optional(),
  blackFrame: z.boolean().optional(),
  heroFrame: z.boolean().optional(),
  alpha: z.boolean().optional(),
});

export const IconConversionOptionsSchema = z.object({
  size: IconSizeSchema.optional(),
  style: IconStyleSchema.optional(),
  frame: IconFrameSchema.optional(),
  extras: IconExtrasSchema.optional(),
});

// Schema for POST body (extras as object, not string)
export const IconOptionsSchema = IconConversionOptionsSchema;

// TypeScript types inferred from Zod schemas
export type IconSize = z.infer<typeof IconSizeSchema>;
export type IconStyle = z.infer<typeof IconStyleSchema>;
export type IconFrame = z.infer<typeof IconFrameSchema>;
export type IconExtras = z.infer<typeof IconExtrasSchema>;
export type IconConversionOptions = z.infer<typeof IconConversionOptionsSchema>;
export type RequiredIconConversionOptions = Required<IconConversionOptions>;
