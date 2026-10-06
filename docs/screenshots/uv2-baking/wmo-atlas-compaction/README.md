# WMO atlas fidelity and packing, 2026-10-06

The pylon previously spent most of its texture area on empty chart rectangles.
Disconnected islands now pack independently, and padded silhouettes interlock
without sharing occupied pixels. Still opaque diffuse and reflection use separate
layouts, so different reflection normals no longer duplicate the same albedo.
Raster reduction scales both axes together; shader sampling retains the original
source UVs and barycentrics. There are no model-name exceptions.

| Pylon measurement | Before | After |
| --- | ---: | ---: |
| Referenced BLP bytes | 36,389,574 | 33,243,854 |
| MDX bytes | 4,808,384 | 4,439,279 |
| MDX + BLP MiB | 39.29 | 35.94 |
| Occupied atlas area, including gutters | 16.78% | 53.47% |
| Texture count | 33 | 33 |

`pylon-before.png` and `pylon-after.png` use the same close-up camera.
`atlas-before.png` and `atlas-after.png` show each export's largest BLP mip 0.
`temple-after.png` verifies the grass-to-brick transition after regeneration.
All are actual model-viewer captures or decoded BLP pixels.

The final pylon export took 121 seconds. Separate passes and higher useful
sampling cost more CPU than the 27-second baseline. Its sampled server heap in
use peaked at 3.61 GiB including loaded CASC; the chart workspace remains capped
at one million samples. Scene lighting and camera-dependent reflections still
use the existing Classic approximation.

Verified: converter/baker and WMO-loader package tests, live pylon and temple
still-bake API integrations, and Firehawk's live-data integration (nine baked
effect sections, eight animated, soft transparency, Classic serialized tracks).
Pylon front/back, temple front/close-up, and Firehawk front/top PNGs were read.
No approved snapshot baselines were changed; WC3 in-game rendering was not tested.

LAN previews:

- [Pylon](http://192.168.0.195:3001/viewer?model=12vd_void_pylon01-wmo-fixed-atlas-compact.mdx)
- [Temple](http://192.168.0.195:3001/viewer?model=12tr_amani_eagletemple01-wmo-fixed-atlas-compact.mdx)

Scripts, manifests, exploratory variants and a portable copy of the final pylon
are in `tmp/wmo-atlas-compaction/`. The temporary capture test is archived there
and removed from the production source tree.

## Lower-teeth correction

The original upper close-up missed a severely undersampled lower batch. Its
raster contained a few islands spanning 111 × 77 texture tiles; the global grid
then reduced most normal triangles to less than one output texel. Integer
rebasing on individual secondary-UV faces also broke otherwise shared edges.

Raster islands now rebase as complete connected components. Islands larger than
one tile have a bounded, uniformly scaled raster footprint; shader sampling still
reads the original UVs and repeat counts. No source image is resized or blurred.
This allocation is shared by WMO shaders and does not check model names.

`pylon-lower-before.png` and `pylon-lower-after.png` use the same camera and expose
the lost/restored lower-tooth detail. `pylon-upper-bounded.png` checks the upper
section. In the lower batch, median mapped triangle area increased from 0.00786
to 14.79 texels, and triangles below four texels fell from 2,705 to 124 of 2,736.
BLP bytes stayed at 33,243,854 (31.70 MiB), MDX fell to 4,410,866 bytes, and the
live export completed in 116.15 seconds. Peak sampled server heap in use was
3,533 MiB including CASC.

The previous 53.47% occupancy included edge padding: mesh texel centres covered
25.58%, with another 27.89% occupied by padding/conservative edge samples. Dense
packing spent freed space on more samples rather than solely smaller atlases.
That is why the earlier BLP saving was only 8.64% (34.70 → 31.70 MiB). Fixed-rate
BLP1 pixel/alpha planes and mipmaps still store empty areas of each power-of-two
atlas. Occupancy is not a measure of detail on the worst surface.

New pylon preview: [bounded raster](http://192.168.0.195:3001/viewer?model=12vd_void_pylon01-wmo-fixed-atlas-bounded.mdx).
The ordinary `12vd_void_pylon01.mdx/.mdl` files also contain this correction.

The regenerated temple integration also passed (249.48 seconds, 44 BLPs,
59,493,100 texture bytes, 6,149,335 MDX bytes). `temple-bounded.png` and its full
front capture retain the masonry/grass transitions. The updated raster checks
and baker/WMO-loader package tests passed; all temporary Go probes/capture tests
are archived under `tmp/wmo-atlas-compaction/scripts/` and removed from source.
