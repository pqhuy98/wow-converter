/**
 * Pipeline switch for WoW model/texture export.
 *  - direct (default): converter parses M2/SKEL/BLP in-process via the raw
 *    file layer (served by the native wow-data-server) and emits MDX + BLP
 *    directly (no OBJ/PNG intermediates).
 *  - legacy: OBJ/PNG export over REST via the wow.export electron app.
 */
export type WowPipeline = 'legacy' | 'direct';

export function wowPipeline(): WowPipeline {
  return process.env.WOW_PIPELINE === 'legacy' ? 'legacy' : 'direct';
}

export function isDirectPipeline(): boolean {
  return wowPipeline() === 'direct';
}
