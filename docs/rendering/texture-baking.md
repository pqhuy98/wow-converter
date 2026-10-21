# WoW shader texture baking

The Go direct M2 converter owns the shared M2/WMO shader baker. Classic WC3 material layers cannot directly represent every WoW texture combiner or UV sample. The baker chooses native layers where safe and otherwise builds combined atlases. [ADR 0002](../decisions/0002-m2-uv2-texture-baking.md) records this ownership.

## Request options

Advanced Textures in Character and Browse exports sends `textureBaking` to `POST /api/export/character`:

```json
{"textureBaking":{"enabled":true,"animate":true,"fps":21,"windowMS":12000,"resolutionScale":0.5}}
```

| Field | Default | Meaning |
|---|---|---|
| `enabled` | false | Opt into shader/particle baking |
| `animate` | true | Preserve animated effects; false bakes a still image |
| `fps` | 8 | Baked sampling rate, accepted range 1–60 |
| `windowMS` | 4000 | Maximum baked loop window, accepted range 1–60000 |
| `resolutionScale` | 1 | Full resolution, or 0.5 for half width and height |

Still mode paints time zero and ignores FPS/window. Native UV translation remains continuous independently of the baked FPS. Local tracks are sampled within each source sequence; global texture loops retain independent WC3 clocks. Particle atlases sample over the emitter's full lifetime rather than a repeatable global window. Equipment and character collections inherit the request options.

Half resolution affects generated textures, including particle atlases, while retaining geometry, normalized UVs, source textures and timing. Straight-alpha images filter in premultiplied space before returning to straight alpha; RGB-only material passes filter independently. The complete export therefore does not necessarily shrink to one quarter of its original size.

## Conversion and size policy

The baker resolves source shader tables, UV sets, texture transforms and combiners. Meshes are rasterized with barycentric interpolation. Conflicting UV islands receive separate charts; shared vertices split only at chart boundaries. Disconnected compatible islands crop separately, retaining source texel proportions. Padded silhouettes pack without sharing occupied samples. Each chart has three texels of padding.

Animated atlases use `DontInterp` UV translation and, for multiple pages, texture-ID tracks. Frame-cell strides align to eight pixels; final pages are powers of two. The page planner minimizes total allocated pixels without shrinking chart detail merely to fit one page.

Default sampling balances temporal frames and spatial detail. Explicit FPS/window requests keep their requested samples and reference spatial detail, so they can use substantially more pages. Native loops shorter than the window retain their duration. Independent loops can reset relative phase at a bounded baked window; native UV loops sharing a source clock use the same window.

Source painted surfaces, effects/environment factors and particles have separate per-model budgets. Sparse chart workspace has a one-million-sample limit, including padding; exceeding it retries at lower spatial resolution. This bounds chart workspace rather than total process memory. Complex WMO batches run at most two workers, decode shared textures once and merge in source order.

Generated texture paths are `baked/uv2/<source-stem>_<12-hex>.png`, with the suffix derived from PNG SHA-256. PNG-identical outputs with matching encoding semantics share files after attachment merge; texture objects retain their wrap and material state. Generated atlases use texture type -1 so component replacements cannot overwrite them. Collection geosets and replacement textures are resolved before baking.

Regular exports write BLP1. Alpha planes choose 0, 1, 4 or 8 bits only when that depth exactly represents every mip's alpha. Known opaque base passes can omit alpha while preserving palette/index RGB. Zero-RGB secondary additive passes can use one black texel; black Blend attenuation textures retain alpha because they still affect the scene.

## Native material paths and limits

Static-mask `Mod_Mod` effects can retain a detailed native second texture and its translation, approximating the first mask with subdivided geoset color/alpha. Opaque or proven cutout cases can retain original triangles and separate native UV draws. Alpha blending separates destination attenuation from additive radiance where the source allows it. The complete sampled mask/UV footprint is checked before selecting a native decomposition.

Subdividing skinned triangles interpolates both positions and bone weights, which can move inserted vertices away from the original posed surface. This remains a visible limitation on some narrow glow/neck details. Validate the posed mesh as well as texture pixels.

Scene lighting, HDR bloom, depth fades, refraction and camera-dependent reflection remain approximations. Environment mapping uses a reference direction; edge fade uses an angle average. Ordinary refraction cards are omitted because distortion needs scene-color sampling. Source particle generators, drag and per-particle random size/lifetime cannot all be reproduced by WC3 PRE2. These paths do not establish full WoW-renderer or in-game WC3 parity.

## Owners and checks

Implementation lives under `golang/internal/converter/wowmodel/direct/m2/` (`texture_bake*.go`, `texture_native*.go`, `wmo_shader*.go`), with WMO orchestration under `direct/wmo/` and encoding in `formats/blp/`. Request defaults are in `internal/config/config.go`.

From the repository root:

```powershell
go -C golang test ./internal/converter/wowmodel/direct/m2 ./internal/converter/wowmodel/direct/wmo ./internal/formats/blp
```

With the converter/data server already running and the desired build loaded:

```powershell
go -C golang test -tags integration_tests ./internal/converter/wowmodel/direct/m2 -run 'TestFirehawkUV2Bake|TestTrollShoulderGlobalUVBakeAnimatedAndStill|TestAdvancedTextureAnimationRealM2Variants' -v -count=1
```

These real-data checks cover Firehawk wing detail/transparency, global-only equipment UV tracks, replacement textures, model variants and serialized animation. They are separate from paired renderer captures and [snapshot checks](../development/verification.md).

`scripts/profile-wmo-texture-bake.ps1` profiles one direct WMO against the existing data server, retaining CPU/heap profiles, MDL and texture SHA-256 manifests. Compare the same source build, request, cache state and format. Count the model plus each distinct referenced texture once; accumulated unrelated export files are not part of its package size.
