# Terrain fitting benchmark

`golang/cmd/terrain-benchmark` compares generated WC3 terrain with the production WoW terrain bake. It writes standalone artifacts without changing exported maps. Production encoding and stock-ID selection are documented in [WC3 tilesets](terrain-tilesets.md).

The suite covers 26 ADTs in five selections: `Azeroth-34_28-36_29`, `Azeroth-32_48-32_49`, `Northrend-27_17-29_18`, `Northrend-29_15-31_16`, and `Voidstorm-32_27-33_29` (map/directory 2771). Voidstorm includes authored material scales 1, 2, 4, 8 and 16.

## Reproduce

From the repository root, with CASC configured in `.env`, initialize captures, geometry, fitting and the 48-family candidate:

```powershell
go -C golang run ./cmd/terrain-benchmark -mode all -paint-families 48 -out exported-assets/terrain-benchmark/aligned -source exported-assets/terrain-benchmark/source
```

Then generate the 63-family control and refresh browser evidence:

```powershell
go -C golang run ./cmd/terrain-benchmark -mode material-fit -paint-families 63 -out exported-assets/terrain-benchmark/aligned -source exported-assets/terrain-benchmark/source
go -C golang run ./cmd/terrain-benchmark -mode views -out exported-assets/terrain-benchmark/aligned
```

For an existing initialized experiment, rerun `material-fit -paint-families 48` and `material-fit -paint-families 63` with the same output/source paths, then `views`. `material-fit` requires the existing report, native meshes and build-matched material captures; `capture` and `materials` resume missing captures. `-region NAME` limits work to one configured selection and preserves other completed regions in the report/evidence. `views` requires an installed Chromium browser and localhost access.

With the converter running, open `/api/assets/terrain-benchmark/aligned/` over HTTP. Use left arrow for WoW and right arrow for WC3, drag to orbit, right-drag to pan and wheel to zoom. **Tile boundaries** opens a full-8K close-up. The 48/63 buttons become available after both scenes are prepared; switching uses cached GPU state and preserves the camera without a loading overlay, network requests or GPU rebuilding. Changing region/focus releases the previous cache. Region overviews use 2K source display textures; close-ups use full 8K references uploaded in quadrants.

## What the comparison holds fixed

The benchmark uses 128 native cells per ADT and 64-pixel subtiles. A 3-by-2 selection uses 384-by-256 cells. Uniform scale 30.72 makes the fixed 3840-unit WC3 height window span 125 WoW units. These settings are independent of production map dimensions, collision fitting and navigation.

`export-48` and `export-63` use the production sheet generator, palette preparation and post-quantization border sealing. Their source references, native meshes, mandatory-model masks, model triangle counts and height windows are identical. The paint-family count excludes the final alpha entry: these consume 49 and 64 total slots respectively. The 63-family control measures texture-budget differences without changing production's stock-ID pool.

Cells touching geometry outside the selected height window become models, including a boundary guard band; source holes remain missing geometry. There is no optional paint-error model budget. Models retain source interior triangles and join native heights at their perimeter. Model textures use coherent source blocks instead of packing unrelated cells together.

## Measurement and visual acceptance

Blending takes priority over RGB accuracy. A favorable SSIM or boundary score alone is insufficient for acceptance.

- Every covered 8K texel contributes to RGB MAE, RMSE, PSNR, channel-error P95, edge error and 8-by-8 luminance SSIM after BLP encoding/decoding.
- Boundary measurement averages 8-by-8 texel blocks and estimates the jump across a cell edge: `1.5 * (right-near - left-near) - 0.5 * (right-far - left-far)`. A linear color slope has zero jump. Mean/P95 result jumps measure continuity; reference jumps and reference-relative errors measure fidelity. Native/native and native/model edges are recorded separately.
- Browser checks cover top, oblique, controlled lighting and full-8K views. CPU/GPU comparisons use decoded BLPs and neighborhoods with several material IDs. Keyboard checks preserve the complete camera state; budget-switch checks also measure stable distinct frames, timing and zero fetch/GPU allocations.

Groundtruth is the production shader-equivalent Go bake with diffuse layers, paint maps, height blending and vertex tint. It is unlit albedo; objects and water are excluded. Metrics use complete 8K references even when the browser overview uses lower-resolution display textures.

Browser/CPU agreement does not establish World Editor or native-game parity. Review cross-ADT edges, mipmaps, road shapes, snow brightness and native/model joins visually. Editor save/reload, terrain hiding, alpha/filtering, lighting, navigation and runtime performance require separate native checks. Terrain BLP bytes are not packed-map size or runtime memory.

## Recorded baseline (2026-10-22)

The retained report uses WoW build `12.1.5.70448` and 1,743,380,480 covered 8K texels across all 26 ADTs. MAE is in RGB channel levels out of 255; edge jump is the mean native/native boundary discontinuity for 48 families.

| Region | Native ground | MAE: 63 / 48 | SSIM: 63 / 48 | Edge jump: 48 |
|---|---:|---:|---:|---:|
| Azeroth-34_28-36_29 | 93.56% | 8.450 / 8.746 | 0.8523 / 0.8495 | 3.988 |
| Azeroth-32_48-32_49 | 93.20% | 6.924 / 7.303 | 0.8846 / 0.8787 | 3.788 |
| Northrend-27_17-29_18 | 61.79% | 7.357 / 7.577 | 0.9104 / 0.9067 | 2.221 |
| Northrend-29_15-31_16 | 34.19% | 6.514 / 6.636 | 0.9344 / 0.9345 | 1.878 |
| Voidstorm-32_27-33_29 | 57.04% | 8.247 / 8.326 | 0.8308 / 0.8296 | 2.203 |

Reducing 63 paint families to 48 adds 0.079-0.379 MAE; the largest SSIM decrease is 0.00590. Against the original four-region 64-family candidate, production encoding reduced native edge jumps by 36-47% while increasing RGB error and lowering SSIM. This is a blending/fidelity tradeoff, not a zero-regression result. `regression-summary.json` retains that comparison; historical expected images were not refreshed.

Model coverage affects fidelity scores, so compare budgets within the same region. High Icecrown has about 65.8% mandatory model area under this fixed window; native ground cannot be the majority without changing its range or scale. Voidstorm's 48-family native/model seam step is 13.773 versus 3.409 in the reference (63 families: 13.186), so that join remains a measured weakness.

## Artifacts

| Artifact | Purpose |
|---|---|
| Source `<ADT>/mesh.json`, `layers.json`, `wow.png`, `preview.png` | Captured geometry, authored material weights and reference albedo |
| Source `materials/<build>/*.png` | Build-matched diffuse/height textures |
| Region `native-mesh.json`, stage assignments/meshes/palette, BLP/PNG sheets and error images | Reproduce geometry, encoded texture sampling and image comparisons |
| `report.json` | Region/stage metrics, source coverage and terrain texture bytes |
| `views.json`, `native-checks.json`, `switch-checks.json` | Browser cameras/scores, CPU/GPU agreement and cached switching evidence |
| `regression-summary.json` | Retained comparison against the original framework |

Use the reproducible viewer and these artifacts for review; static screenshots in this guide are not maintained evidence.
