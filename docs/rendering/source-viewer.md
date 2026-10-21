# Original WoW renderer viewer and captures

The converter embeds the WebWowViewerCpp WebAssembly renderer and shaders. It
loads raw M2, WMO roots/groups, skin, animation, skeleton and BLP assets from the converter's loaded
CASC client. No wow.tools.local server or installation is needed.

For interactive viewing, open `/wow-viewer?model=spells/acidliquidbreath.m2`
or click the external-link icon in the Browse model viewer. The page plays
source animations and particles continuously, with animation selection, pause
and camera reset. Camera gestures match the converter viewer: left drag rotates,
right drag moves along the ground, middle drag or left+right pans, wheel zooms,
and touch supports rotation and pinch zoom. The source model must be an M2 or WMO root in
the loaded CASC client; numeric file IDs also work.
Initial framing and camera reset use the converter's exported geometry extents
and its default orbit rule. Native loading/autofit frames stay hidden until
that camera has reached the draw pipeline.
The Browse link carries the completed preview's selected skin as `&skin=…`.
The WoW viewer also offers a Skin selector; switching texture variants preserves
the camera and animation. Without a skin parameter, it uses the first available
skin. For example: `/wow-viewer?model=creature/ghoul2/ghoul2.m2&skin=ghoul2grey`.
WMO models use the same Browse icon and orbit gestures through the native
first-person camera adapter. Their group geometry, textures and doodads load
from CASC; animation and skin selectors appear only where supported.

Run the converter, then from the repository root:

```sh
go -C golang run ./cmd/shot-wow-source spells/acidliquidbreath.m2 --seq Stand --view left --time 2
```

A numeric file ID also works. Output is a 1440x900 canvas PNG under `tmp/shots`.
The sequence is an exact WoW animation name; the native renderer selects that
animation ID's primary variant. `--time` replays a fixed duration at 60 Hz,
including repeated animation loops, then freezes the model and particles.
Omit it to use the source animation midpoint. Blank frames and failed asset loads
fail the command; use a nonzero time for effects that start empty.

For a freshly exported WC3 effect:

```sh
go -C golang run ./cmd/shot-wow-source 165552 --model acidliquidbreath.mdx --converter-seq "Stand 1" --view left --time 2 --out tmp/effect-pairs
```

`--model` is relative to `exported-assets`. Select the WC3 sequence from the
export's `reportMetadata.models[].sequences` so its `wowName` and `wowVariant`
match the source. Pass the export's `modelScale` with `--model-scale` when it
differs from the default 56. Both images use the same camera and elapsed time.

For an existing Wowhead comparison, pass the saved camera map with `--cameras`.
The source host inverts the reference model matrix and compensates for the native
renderer's orbit convention and its sequence-zero bounds center. The existing
Wowhead and converter camera algorithms remain the reference.

Other options: `--base`, `--textures` (replacement texture IDs indexed by M2
texture type), `--view front,left,back,right,top,bottom` or `all`, and `--sheet`
(six-view source sheet, plus a converter sheet when paired).

The WC3 particle conversion preserves emitter type, angular ranges, scalar or
compressed gravity, and source trail durations/rates. PRE2 cannot represent every
M2 generator exactly: visible spherical shells and independent angular ranges
use at most 32 deterministic sectors while conserving the total emission rate.
The M2 loader also reads EXP2 source-point overrides; static positive plane
source points become downward fans. Compressed gravity's vertical component is
retained; PRE2 has no lateral acceleration track. Drag, variable lifetimes and
per-particle random sizes remain approximations, so particle density and trail
details can differ even with the same camera and elapsed time.
