# Native WC3 terrain tilesets

Enable **Terrain as WC3 tilesets** in the export dialog; it defaults off. Production fitting uses the final heightfield dimensions and actual ADT placement transform. W3E rows increase world Y. Collision fitting and liquid pathing are described in [navigation](terrain-navigation.md) and [water](water.md).

## Palette and stock IDs

`golang/internal/converter/mapexporter/terrain/terrain_tileset.go` owns `wc3GroundSlots`. Its length sets the production paint-family budget: currently 48 paint families plus native `Maph`, or 49 palette entries. The budget is not exposed in the export UI. Increasing it requires adding eligible stock IDs, not changing a standalone count.

The pool contains 40 least-used non-cliff IDs allowing building, ground movement and flight, plus eight allowing ground/air movement but not building. After model interiors become `Maph`, export counts final W3E corners per fitted family and assigns any required non-buildable IDs to the smallest populations. Fits with at most 40 families use unrestricted IDs only. Palette ordering groups similar material/color swatches independently of stock-ID assignment.

Export replaces existing `Terrain.slk` IDs through their stock BLP paths. A map-imported `Terrain.slk` cannot add IDs or change stock World Editor tile pathability defaults. WPM and runtime `SetTerrainPathable` control locations, while the selected IDs retain their stock properties after editor painting/regeneration. Structural blockers remain independent.

## Texture encoding and blending

Fit continuous authored paint mixtures, tint and material detail; do not stamp unrelated baked world fragments into cells. Modern source texture scales use UV-phase features within the same family budget, so their reproduction remains an approximation.

Extended 512×256 sheets use variations 0–15 for full variants in layers 16–31; variation 16 selects full-mask layer 15. Every regular brush frame is filled. `Maph` is last and uses the game's native WC3 3.0 alpha tile without a texture import.

The 16 full variants vary source detail while matching family tint/contrast and periodic RGB borders. Borders are sealed after palette quantization at each complete atlas mip. Generated paint preserves authored repeating detail phase; the additional variants are available to the editor brush. Transition contours follow source height/detail inside a tile while keeping shared alpha edges fixed. Sequential alpha overlays and doubled corner influence prevent unrelated lower layers leaking through shared edges. These constraints preserve continuity but do not make the large-scale grid disappear in every material.

## Native ground and model boundaries

ADT models belong only to unplayable height-limit cells, never chunk cracks or paint-error patches. Their cells are blocked for ground pathing, with native alpha paint hiding the underlying terrain. The original source tree remains available for bounds, water, navigation and minimaps.

Model edges match final native heights. On the model side, albedo crossfades over one cell into hypothetical WC3 paint sampled at the same world position. Transition patches use 64 pixels per cell with gutters. Native/model joins remain a visual limitation; see [benchmark measurements](terrain-benchmark.md#recorded-baseline-2026-10-22). Browser agreement alone does not establish World Editor parity.

## Stock selection data

The extracted pre-2.0 WC3 catalog contains 177 `Terrain.slk` rows and 36 `CliffTypes.slk` rows. Exclude every nonnegative `cliffSet` (all cliff-base groups), every cliff ground/upper tile link, and every row disallowing walking or flight. This leaves 53 unrestricted and 51 non-buildable candidates. These are properties of that catalog, not a verified inventory of current WC3 metadata.

From the repository root:

```powershell
bun scripts/analyze-melee-terrain.ts <extracted-mpq> exported-assets/terrain-benchmark/melee-tile-usage 40 8
bun scripts/analyze-melee-terrain.ts --self-test
```

The 2026-10-22 scan found 561 archives: 242 distinct melee terrains counted, 303 non-melee maps skipped, 16 duplicate W3E payloads skipped, and no parse failures. Maps overriding stock terrain textures are excluded; exact terrain duplicates count once. Different seasonal geometry remains separate.

Rank painted W3E corners inside playable bounds, then full-map painted corners, then ID. Palette membership alone is not usage. This measures paint assignments, not alpha-weighted visible area. The analyzer supports W3E v11/v12 and W3I prefixes v18/v25/v31.

Outputs are `report.json`, `tiles.csv` and a Go slot declaration. `golang/internal/converter/mapexporter/terrain/testdata/stock_ground_candidates.json` freezes the catalog properties and ranked usage for regression tests. Production pool changes do not alter the benchmark's independent sampling/budget; see [reproduction](terrain-benchmark.md#reproduce).
