/** Local development mode (NODE_ENV=development). */
export function isDev(): boolean {
  return process.env.NODE_ENV === 'development';
}

/** Bundled single-binary / in-process wow-data-server mode. */
export function isBundledEnv(): boolean {
  const v = process.env.WOW_CONVERTER_BUNDLED ?? process.env.WOW_CONVERTER_BUNDLE;
  return v === '1' || v === 'true' || v === 'yes';
}
