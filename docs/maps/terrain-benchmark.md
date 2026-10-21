# Terrain fitting benchmark

`golang/cmd/terrain-benchmark` compares generated WC3 terrain with the production WoW terrain bake. It writes standalone artifacts and leaves exported maps untouched. The four selections contain 20 ADTs: `Azeroth-34_28-36_29`, `Azeroth-32_48-32_49`, `Northrend-27_17-29_18`, and `Northrend-29_15-31_16`.

## Reproduce

From the repository root, with CASC configured in `.env`:

```powershell
go -C golang run ./cmd/terrain-benchmark -mode all -out exported-assets/terrain-benchmark
```

To reuse the existing aligned experiment and its source captures:

```powershell
go -C golang run ./cmd/terrain-benchmark -mode materials -out exported-assets/terrain-benchmark/aligned -source exported-assets/terrain-benchmark/source
go -C golang run ./cmd/terrain-benchmark -mode material-fit -out exported-assets/terrain-benchmark/aligned -source exported-assets/terrain-benchmark/source
go -C golang run ./cmd/terrain-benchmark -mode views -out exported-assets/terrain-benchmark/aligned
```

`all` captures source images, geometry and original material data, runs the historical fit/refinement, then builds the authored-material candidate and browser comparisons. `capture` and `materials` resume build-matched captures. `fit` creates the historical baseline; `refine` resumes its geometry work. `-material-region NAME` or `-view-region NAME` limits a focused iteration. `views` needs an installed Chromium browser and localhost access.

With the converter running, open `/api/assets/terrain-benchmark/aligned/`. The HTML needs HTTP because it loads adjacent files. Use ← for WoW and → for WC3, drag to orbit, right-drag to pan, and wheel to zoom. **Tile boundaries** opens a full-8K close-up. Switching stages preserves the camera.

## Current representation

The experiment uses 128 native cells per ADT and 64px subtiles. This aligns with WoW's 128 diffuse repeats per ADT for the captured scale-1 materials. A 3×2 region uses 384×256 cells. Uniform scale 30.72 makes the fixed 3840-unit WC3 height window span 125 WoW units. These are experimental settings, not a saved map's export configuration.

The 64-family budget targets Warcraft III 2.0.3 or later. [Blizzard's patch notes](https://us.forums.blizzard.com/en/warcraft3/t/warcraft-iii-reforged-patch-notes-version-204/36567/4) increased the tileset limit from 16 to 64 and changed the terrain file version. Production integration must write that compatible W3E version and verify editor round-trips.

The `material-blend` candidate captures original diffuse/height textures, authored paint weights and vertex tint. It clusters paint mixtures and tint into at most 64 families, then builds repeatable textures using the production height-blend function. It uses shared corner labels, sorted material priorities, 16 transition masks and 16 identical full variants. It does not stamp independently chosen baked world fragments into cells. Non-unit source texture scales currently stop fitting because they need explicit phase allocation.

Cells touching geometry outside the selected height range become models, including a boundary guard band. Source holes remain missing geometry. There is no optional error-cell model budget in this candidate. Model textures use coherent 512px source blocks, avoiding mipmaps that mix unrelated packed cells. Model interiors retain original triangles; only their perimeter is stitched to native heights. Tiny float32 UV errors are snapped to the intended ADT grid before sampling and clipping.

## Measured candidate

WoW build `12.1.5.70448`, 20 ADTs and 1,340,833,792 covered 8K texels. These are the authored-material candidate scores after encoded BLP decoding; visual review remains a separate gate.

| Region | Native ground | MAE / 255 | SSIM | Terrain BLP MiB |
|---|---:|---:|---:|---:|
| Azeroth-34_28-36_29 | 93.56% | 7.84 | 0.8818 | 72.9 |
| Azeroth-32_48-32_49 | 93.20% | 6.53 | 0.9214 | 43.5 |
| Northrend-27_17-29_18 | 61.79% | 6.94 | 0.9339 | 250.5 |
| Northrend-29_15-31_16 | 34.19% | 6.48 | 0.9442 | 394.0 |

Higher model coverage also raises source-image fidelity, so comparisons between regions must account for the native/model split.

## Why the earlier result had square patches

The rejected `clamp-hybrid` / `seam-first` approaches chose local baked texture fragments and optional model cells by image error. Roads, dark spots and shading could therefore appear in unrelated cells. Conditioning narrow borders lowered a metric without making the interiors coherent. Those stages remain as rejected comparison history; the smoothing generator was removed.

The first authored-material attempt exposed a second issue. The historical renderer copied HiveWE's backward recursive RGBA calculation. With three or four materials, its accumulated alpha can let an unrelated lower-priority corner show through at a shared edge. The newer candidate uses sequential overlays, each weighted by its own alpha. This matches the [mdx-m3-viewer ground shader](https://github.com/flowtsohg/mdx-m3-viewer/blob/master/src/viewer/handlers/w3x/shaders/ground.frag.ts) and the blending order in the [reverse-engineered WC3 terrain shader](https://github.com/FernandoS27/Blizzard-Shaders/blob/master/wc3_shaders/terrain_ps.slang). It still needs a controlled native-game check.

Bilinear equal-ID masks alone also leave gaps between overlapping materials. The new custom masks use `min(1, 2 × corner influence)`: at a shared edge, at least one of the two edge materials fully covers any unrelated lower layer. An exhaustive regression checks RGB continuity for all 4,096 two-cell assignments of four material IDs. Historical assignments retain their original calculation so cached images and scores remain reproducible.

## Measurement and visual acceptance

Blending takes priority over RGB accuracy. A favorable SSIM or boundary score is insufficient for visual acceptance.

- Every covered 8K texel contributes to RGB MAE, RMSE, PSNR, channel-error P95, edge error and 8×8 luminance SSIM, after BLP encoding and decoding.
- Boundary measurement averages 8×8 texel blocks and estimates the jump across a cell edge: `1.5 × (right-near − left-near) − 0.5 × (right-far − left-far)`. A linear color slope has zero jump. Mean result jump and P95 measure continuity; source jump and source-relative error separately measure fidelity. Native/native and native/model edges are separate. Cross-ADT edges and mip behavior require visual review.
- Browser captures include top, oblique, controlled lighting and full-8K close-up views. CPU/GPU comparisons use actual decoded BLPs, including a neighborhood with several material IDs. Keyboard checks verify that ← / → keeps the complete camera state unchanged.
- `report.json` stores region/stage metrics and texture bytes; `views.json` stores camera matrices and screenshot scores; `native-checks.json` stores CPU/GPU comparisons. The HTML renders native tiles and source/model geometry rather than displaying a pre-rendered WC3 result image.

Groundtruth is the production shader-equivalent Go bake, including diffuse layers, paint maps, height blending and vertex tint. It is unlit albedo, not a WoW game-client capture. Objects and water are excluded. Individual ADTs load full 8K source images; region overviews use 2K display textures to bound memory. All albedo scores use the complete 8K references.

This remains an experimental renderer comparison. Road shapes, color quantization, model/terrain joins and minification need visual scrutiny. Native Warcraft III save/reload, terrain hiding beneath models, alpha/filtering, lighting, navigation and performance remain unverified. High Icecrown terrain has about 65.8% mandatory model area under this fixed clamp, so native ground cannot be the majority there without changing the range or scale. File totals describe terrain BLPs, not packed maps or runtime memory.

Historical renderer reference: [HiveWE terrain assignment](https://github.com/stijnherfst/HiveWE/blob/master/src/base/terrain.ixx) and [shader](https://github.com/stijnherfst/HiveWE/blob/master/data/shaders/terrain.frag), locally inspected at commit `0e58c495515b2b18b3e40e289ea0485848e31be9`.
