# WoW shader texture baking

The Go direct converter owns shader baking. Classic WC3 has one UV set and
framebuffer material layers, so independently addressed WoW texture samples
need a combined bake or an equivalent native decomposition. The TypeScript request
schema also describes the Go API options; the legacy TS renderer is not the
baking implementation.

The baker resolves the M2 effect/pixel/vertex tables and WMO shader tables,
samples their source UV sets and texture transforms, and evaluates their
combiners. Meshes are rasterized with barycentric interpolation. Conflicting UV
islands get separate charts; shared vertices are duplicated only at chart
boundaries. Cropped charts have three texels of padding. The fragment baker
preserves positions, skinning, normals and native model animation. Raster UV
selection uses triangle texture coverage, rather than overall UV bounds.
Spatial reduction preserves the chosen source texel proportions; atlas packing
shape must not repeatedly shrink just one surface axis.

For depth-free `Mod_Mod` effects with a static first sample, the detailed second
texture and translation loop remain native at source resolution. Conforming
triangle subdivision approximates the first sample through geoset color/alpha.
The complete source footprint is checked, including fractional bilinear edges;
sparse vertex samples cannot silently discard an interior feature. RGB mask
error uses a 1/16 footprint bound plus 1/16 tint sharing for additive radiance,
or 1/255 tint sharing for blended radiance, before batch tint/visibility. Native opacity uses
1/64 plus half an alpha bucket (1/256). Square-root RGB buckets preserve faint
effects. Similar quantized tints share draws.
Complex masks fall back at the subdivision, face or draw limit. Refinement
subdivides faces and propagates splits to adjacent faces. New vertices
interpolate normals and skin weights. Dimensius's neck still exposes a limitation
of this approximation: interpolating both positions and changing skin weights
can warp a subdivided triangle into its underlying depth-tested surface. A
prototype preserving original triangles removed the gaps but blurred approved
star detail and increased asset size. That prototype is parked rather than
enabled; see the [neck investigation checkpoint](../screenshots/uv2-baking/dimensius-detail/neck-investigation.md).

Opaque/cutout `Mod_Mod` faces whose first sample is fully opaque and nearly
uniform can use the same detail texture in one native draw. Their original
triangles are retained, avoiding seams with the baked remainder. Variable mask
coverage or detailed first-sample artwork still uses the combined bake. For
cutouts, detail alpha is remapped from [0,1] into [0.5,1] to match Classic's
0.75 test to WoW's approximately 0.5 test, with byte quantization at the edge.
This path requires full, static batch visibility.

Opaque `Mod_Mod` surfaces can also multiply the two original textures with two
native UV draws, retaining original triangles and both source resolutions.
Cutout faces use this path only when the first sample is fully opaque and the
second stays above the cutout threshold throughout its complete translation
sweep. This proof prevents modulation of background pixels through cutout
holes. Other faces retain the combined bake. The modulation draw has neutral
alpha and finishes before alpha/additive effects. This path requires the source
surface to write and test depth, with full, static batch visibility.

Alpha blending separates destination attenuation from additive radiance. An
opaque swept detail domain permits an exact static alpha texture; otherwise
native alpha groups retain the same detailed source loop. Alpha-only baking is
the bounded fallback. Source batch priority is signed, matching the M2 skin
format. Opaque environment modulation finishes before transparent effects.

Static effects use one frame. Animated effects use `DontInterp` UV translations
and, when an atlas needs multiple pages, animated texture IDs. Global texture
loops retain independent WC3 global sequences. Local tracks are sampled within
each native animation interval. Long loops repeat a bounded source window;
independent global tracks can reset their relative phase at that boundary.
Native UV loops sharing a source clock with a baked material repeat the same
window, preserving source velocity inside it; independent native loops retain
their full duration. This keeps flowing detail aligned with baked cutouts.
Baked paths are `baked/uv2/{wow-stem}_{12-hex}.png`. The stem is the listfile
basename. The 12 hex digits are SHA-256 of the PNG. Identical atlases of the
same WoW file share one BLP. The regular exporter writes BLP1.

Emission is separated from diffuse shading. Opaque environment-metal materials
retain their full-resolution albedo and use a baked modulation layer for their
environment factor. Three-texture particle effects and anisotropic sprites are
baked over particle age into native PRE2 atlases with deterministic phase
variants. Particle lifetime is preserved.

## Request options and size policy

Baking is off unless the request sets `enabled`. `POST /api/export/character`
accepts:

```json
{"textureBaking": {"enabled": true, "animate": true, "fps": 21, "windowMS": 12000}}
```

`enabled` defaults false. `animate` defaults true when omitted. `fps` is 1 to 60.
`windowMS` is 1 to 60000. Omitted fps/window use 8 FPS and a 4000 ms maximum
baked window. `animate: false` paints one combined texture at time 0 and does
not attach a flipbook; fps/window are ignored. Native UV translation is
continuous and does not use this FPS. Native loops shorter than the window
retain their duration. Static effects do not acquire animation. PRE2 sampling
covers the full particle lifetime at the selected FPS, because its atlas
advances over lifetime rather than a repeatable global texture loop. Still-frame
mode bakes particles at age 0.

Painted opaque/cutout surfaces share an 8 × 1024² reserve by physical area;
effects and environment factors share a separate 2 × 1024² reserve. Models with
only effects use the previous 6 × 1024² reserve. Particles have a 2 × 1024²
budget. Atlas paging is considered before shrinking detail merely to fit a
complete flipbook into one 2048-pixel page. The baker reduces samples before shrinking
already-small surface charts. Explicit settings retain that baseline spatial
detail and honor their requested temporal samples, using extra atlas pages as
needed. They can exceed the compact budget substantially. This makes FPS/window
comparisons meaningful without silently lowering FPS or blurring high-FPS
variants. Power-of-two atlas padding can make size changes discontinuous.

## Limits

WoW scene lighting, HDR bloom, depth fades, refraction and camera-dependent
reflection cannot be reproduced exactly by Classic material layers. Environment
coordinates use a reference direction; edge fade uses neutral coverage rather
than baking a camera-specific silhouette into the model. Deferred character
textures retain the existing conversion until source pixels are available.
This covers the renderer's known shader tables, not full WoW runtime parity.

## Verification

From `golang`, run `go test ./internal/converter/wowmodel/direct/m2` for combiner,
UV-chart, timeline and spatial-budget checks. The explicit-settings regression
checks all six 12/21/30 FPS × 4000/12000 ms combinations under a small pixel
budget and verifies every generated frame key.

With the existing data server running, run:

```
go test -tags integration_tests ./internal/converter/wowmodel/direct/m2 -run TestFirehawkUV2Bake -v -count=1
```

The Firehawk integration checks transparency, material flags and serialized
Classic flipbook tracks and writes MDL/MDX. `UV2_TEST_OUTPUT` retains its files.
Use the Go screenshot commands to compare actual exports with Wowhead. Preview
validation uses mdx-m3-viewer; in-game WC3 validation remains separate.
