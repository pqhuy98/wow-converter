# Dimensius detail comparison

Source: https://www.wowhead.com/npc=233824/dimensius (M2 file 6789802).
Captured 2026-10-05 with isolated Edge and the screenshot skills' camera
transfer. These are review evidence, not approved snapshot baselines.

The source uses `Stand`; the converter uses `Stand 1`. Both hold the stand
pose at its midpoint. The chest uses texture time 2917 ms. The base uses
texture time 0 ms, viewed from above at the transferred source camera. The
front pair uses the standard Go screenshot command and camera transfer.

The converter images are from `uv2-dimensius-detail-preview.mdx`, Classic
MDX800, with `textureBaking: {fps: 21, windowMS: 4000}`. Translation loops
that remain native are continuous. Related native/baked clocks share the
bounded window; independent native clocks retain their original duration.

| Export | MDX bytes | Referenced BLP bytes | Total MiB |
| --- | ---: | ---: | ---: |
| `uv2-dimensius-detail-preview.mdx` (4 s) | 12,863,027 | 190,278,740 | 193.73 |
| `uv2-dimensius-detail-preview-12s.mdx` (12 s) | 12,911,703 | 377,637,274 | 372.46 |

Sizes count unique texture paths referenced by each MDX, excluding MDL,
PNG, and unrelated exports. Both exports contain 267,909 triangles and 207
geosets; their geometric cost is higher than the earlier combined bake.

The stars retain source texture resolution. The main body uses two native
UV draws on 3700 of its 5416 original triangles, with swept alpha coverage
proved before factorization. Remaining coverage edges are baked. Skull
faces on the base are substantially clearer, but the baked/native boundary
can remain visible. The central orb still differs in sharpness and glow;
this is not a claim of complete WoW rendering parity or in-game validation.

Verification: the direct M2 converter, M2 parser, bundle metadata, MDL and
BLP package tests passed, as did `TestFirehawkUV2Bake` against the existing
data server. The final viewer URLs and MDX assets respond over the LAN.
