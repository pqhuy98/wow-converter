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
boundaries. Disconnected compatible islands are cropped separately. Their padded
silhouettes interlock in a deterministic skyline packer; overlapping bounding
rectangles never imply overlapping sampled texels. Charts retain three texels
of padding. The fragment baker
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

Final pages remain powers of two, but repeated frame cells need not be. After
choosing spatial detail with the same reference budget, the exporter crops only
unused space beyond the padded chart bounds and aligns cell strides to eight
pixels. It searches uniform page dimensions for the smallest total pixel count,
including incomplete final pages. UV translations use cell strides divided by
page dimensions; texture-ID tracks switch pages at those same frame indices.
Chart samples, three-pixel gutters, FPS, and capture duration are retained.

`textureBaking.resolutionScale` accepts `1` (default) or `0.5`. Half resolution
averages straight RGB and alpha separately at final texture registration, halving
width and height while retaining normalized geometry UVs and all timing tracks.
It affects generated M2/WMO/particle textures in still and animated modes,
including character attachments. Native source textures are unchanged, so the
complete asset package does not necessarily shrink to one quarter its size.

PNG-identical generated pages are reused within a model. After attachments are
merged, identical PNG files with matching encoding semantics share a file path
without merging wrap flags or other texture-object state. BLP1 alpha storage uses
the smallest depth that reproduces every alpha value in every mip exactly:
0 bits for fully opaque output, 1 for binary, 4 for multiples of 17, otherwise 8.
Opaque baked base layers additionally mark alpha unused, removing that plane
after color quantization without changing any palette/index or mip RGB bytes.
This flag is distinct from forcing source PNGs opaque and has a separate hashed
filename. Blended attenuation masks always retain their alpha. Zero-RGB secondary
additive passes may use a single black texel because they never write depth.

Temporary UV-chart samples are sparse: conflicting islands store only occupied
texels rather than a full raster grid per island. Padding adds only the three
boundary waves and retains deterministic neighbor selection. Chart construction
and padding share a one-million-sample working limit; if a raster exceeds it,
the baker retries at lower spatial resolution before allocating more samples.
This limit is separate from RGBA atlas budgets and also applies to still frames.
It bounds chart workspace, not the process's CASC, source-texture or model data.

Unused M2 batch reserves are redistributed to subsequent batches. Native painted
textures therefore do not strand most of the painted-surface allowance while
dense effect cards receive only a few texels per triangle. Usage is measured at
the common compact reference timeline, before explicit FPS/window expansion, so
temporal comparisons keep the same spatial detail.

Character equipment and collection meshes resolve replacement textures by M2
texture type before shader baking. Global UV tracks retain their own timestamps
and do not require an ordinary model animation. Generated atlases use texture
type -1 so later component-0 replacement cannot overwrite them. Source component
0 remains a valid replacement slot.

Collection customization geoset IDs are selected before mesh construction and
shader baking, using the existing exact-ID filter. Equipment collection filters
also run early; models shared by equipment slots retain the union needed by all
slots. Final collection filtering remains in place. This avoids baking hidden
variants without losing parts required by later template reuse.
Nonempty submesh 0 is always retained alongside the selected extras. A requested
suffix-zero variant does not enable an arbitrary sibling when absent; fallback
for missing nonzero variants stays within the requested groups. Exact selections
are preserved regardless of vertex count.

Chart construction indexes only overlapping raster samples and prepares still
shader colours lazily for actual conflicts. Faces reuse raster-write buffers;
WMO varyings and face tangent bases are cached. Padding expands the boundary
frontier while retaining the original neighbour priority and three-wave halo.

Complex WMO exports bake at most two independent batches concurrently. Source
textures are resolved and decoded serially once; workers have private mutable
model graphs. Source-order merging preserves the serial geometry budget and
output ordering. Generated animation references are rebased once and resolved
against the final animation slice after all merges. Smaller exports remain
serial. This bounds simultaneous chart workspace; CASC stays in the existing
data-server process.

## Limits

WoW scene lighting, HDR bloom, depth fades, refraction and camera-dependent
reflection cannot be reproduced exactly by Classic material layers. Environment
coordinates use a reference direction; edge fade uses a spherical viewing-angle
average rather than baking a camera-specific silhouette into the model. For
WoW's `clamp(2.7*max(dot(view,normal),0)^2-.4,0,1)`, the mean is 0.215282.
Alpha-blended diffuse also preserves the second moment (0.187864), since WoW
fades both mesh RGB and opacity. This softens shells from every angle; it cannot
retain their exact camera-dependent outline or lighting. Deferred character
textures retain the existing conversion until source pixels are available.
This covers the renderer's known shader tables, not full WoW runtime parity.

## Verification

From `golang`, run `go test ./internal/converter/wowmodel/direct/m2` for combiner,
UV-chart, timeline and spatial-budget checks. The explicit-settings regression
checks all six 12/21/30 FPS × 4000/12000 ms combinations under a small pixel
budget and verifies every generated frame key.

WMO still-frame bake coverage is the retail snapshot cases
`troll-12tr_amani_hub03`, `void-12vd_void_pylon01`, and
`troll-12tr_amani_eagletemple01` (`textureBaking.enabled`, `animate: false`,
`resolutionScale: 0.5`). Direct conversion profiling of the hub remains
`go test -tags integration_tests ./internal/converter/wowmodel/direct/wmo -run TestProfileWMOTextureBake -v -count=1`.

Still WMO baking uses four times the effect spatial budget, capped at one
megapixel per material. Missing shader-23 environment textures generate no
empty emissive pass. Unrelated subpixel UV islands retain separate conservative
samples; faces sharing an edge can still share boundary coverage. Sparse chart
working-memory limits remain in place.

Still opaque WMO diffuse and emission use independent layouts, reserving three
quarters of the material budget for diffuse detail and one quarter for emission.
Different reflection normals no longer force identical albedo into duplicate
charts. Still conflicts compare the painted result, ignoring masked-out inputs;
opaque RGB reuse requires identical clamped eight-bit output. Animated and
alpha-dependent materials keep their combined sampling/coverage constraints.
Disconnected tiled raster islands drop their whole integer tile offset. Islands
spanning more than one tile scale uniformly into a bounded raster footprint;
one extreme repeated UV island cannot shrink the common grid for all others.
Secondary UVs rebase the complete connected island instead of individual faces,
retaining shared edges across tile boundaries. Shader inputs keep the original
UVs and repeat counts. Shorter islands retain their source raster density.
Budget reductions scale both raster axes together; raster grids need not be
powers of two, while final WC3 atlases remain powers of two.

WMO shader inputs keep three colour streams distinct: first MOCV is lighting,
second MOCV is two-layer blend alpha, and MOC2 supplies four-layer weights/AO
or parallax coordinates. The baker reverses the loader's OBJ V flip before
sampling BLP pixels. Blended faces preserve a usable active primary unwrap;
otherwise they rasterize through the most detailed active source UV layer.
Shader sampling still uses the original four UV sets. This
prevents unused or collapsed UV1 from flattening detail stored in UV2/3/4.

With the existing data server running, run:

```
go test -tags integration_tests ./internal/converter/wowmodel/direct/m2 -run TestFirehawkUV2Bake -v -count=1
```

The Firehawk integration checks transparency, material flags and serialized
Classic flipbook tracks and writes MDL/MDX. `UV2_TEST_OUTPUT` retains its files.
Use the Go screenshot commands to compare actual exports with Wowhead. Preview
validation uses mdx-m3-viewer; in-game WC3 validation remains separate.

`TestTrollShoulderGlobalUVBakeAnimatedAndStill` exercises a real global-only
equipment UV track with explicit component replacements, checking both animated
and still export and MDL reload. WMO worker tests compare exact texture bytes and
optimized serialized models, including several animated passes and pre-existing
animation references.

`TestAdvancedTextureAnimationRealM2Variants` checks the Akilzon, Aether Serpent,
Elemental Primalist and Thunder Lizard skin variants and serialized animation.
The elemental base rotates on the source 2667 ms global bone clock; global bone
keys retain their raw timestamps and do not follow local sequence offsets.
M2 and SKEL loop durations are retained as u32 values. Baked particles retain
the same world-unit scale as native emitters. Ordinary refraction cards are
omitted because their distortion textures require scene-color sampling; a
multi-texture emitter retains its normal color path when both flags are set.

From the repository root, `scripts/profile-wmo-texture-bake.ps1` profiles one
direct WMO conversion against the existing data server. It writes CPU/heap
profiles, a sampled peak-heap report, MDL and texture SHA-256 manifests. Detailed
measurements and actual captures are in
`docs/screenshots/uv2-baking/texture-bake-performance/README.md`.
