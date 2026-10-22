# Collision terrain and path blockers

`golang/internal/converter/mapexporter/terrain/terrain_navigation.go` fits native ground to NPC-reachable collision surfaces without changing the chosen clamp or map/object scale. It runs independently of NPC model export. [Liquid pathing](water.md) is separate; [tileset models](terrain-tilesets.md#native-ground-and-model-boundaries) are restricted to unplayable height-limit cells.

**Place path blockers** is available when selected cells have supported NPC data. It defaults on in the UI; direct Go/API configuration defaults off. With it off, terrain fitting and JSON/PNG diagnostics still run, but structural blocker doodads/pathing are omitted. Liquid pathing can still write WPM. Blocker types enable World Editor's click helper (`duch=1`).

## Surface fitting

ADT faces and transformed WMO/M2 collision rasterize into 32-unit pathing cells. WMO MOPY rules include collision-only faces outside render batches; M2 uses collision rather than visual geometry. Bulk/map conversion retains these source triangles. Spawn/event filtering is shared with [placement](README.md#gameobjects-and-scenery).

NPC feet seed the search only when nearby collision supports standing within `seedSnap`. Airborne/unsupported spawns do not seed it; this uses geometry, not creature flight flags. With no grounded seeds, or `navigation.disabled`, generation retains ADT-derived ground. Newer client geometry can disagree with Wrath spawn positions.

The multi-source search retains vertical spans, checks floor slope, steps, body radius/clearance, drops and parabolic jumps. Upward faces provide floors; undersides obstruct clearance. Jump-only routes become walkable native ground, with short gaps filled by invisible ramps. This sampled static traversal does not simulate moving platforms, scripted doors or swimming.

Overlapping reachable components are selected by supported NPC count, then shorter search distance, then lower height. Native corners sample the winning collision plane, including narrow roads. WC3 supports one ground height per XY; projection conflicts and wall footprints are blocked rather than joining different floors. Collision outside the selected clamp cannot become playable ground.

## Structural blockers

Unreachable cells block walking/building. Collision above `airBlockHeight` relative to nearby reachable ground also blocks flight. WPM and custom doodad types/instances (W3D/DOO) encode the result; blockers are not destructibles. Their model is template `empty.mdl`, containing no geometry/nodes.

Packing uses completely solid square footprints. Four disjoint scan orders compete with a largest-first same-class overlapping cover; the fewest-instance result wins. This is greedy, not a minimum-cover guarantee. Types are shared by size/blocking class. Minimum footprints are 2×2 pathing cells; larger sizes are even. Stock footprints are reused where available; other sizes are uniform BGR TGAs under `pathing/square-*.tga`.

Facing/fixed rotation are 270°, cancelling WC3's additional 90° pathing rotation. Native save measurements show centers snap on a 64-unit grid, offset by 32 units when integer half-size is odd. The exporter conservatively combines 2×2 cell blocks before packing, so snapping cannot move a footprint into clear ground. Thin boundaries can gain one blocked cell. WPM/PNG use the same adjusted grid; exact arbitrary 32-unit silhouettes are outside this representation.

## API configuration

The optional `navigation` object on `POST /api/maps/{map}/generate-wc3` overrides individual defaults. Distances are source WoW units; slope is degrees. Omitted values retain defaults; explicit zero disables jumping/stepping where applicable.

```json
{
  "placePathBlockers": true,
  "navigation": {
    "disabled": false,
    "radius": 0.35,
    "height": 1.8,
    "maxSlopeDegrees": 45,
    "stepHeight": 0.6,
    "jumpHeight": 1.5,
    "jumpDistance": 3,
    "maxDrop": 3,
    "seedSnap": 1.25,
    "airBlockHeight": 6
  }
}
```

## Diagnostics and validation

`terrain-navigation.json` records seeds, coverage, packing, conflicts and median NPC height errors. `traversalReachableCells` precedes projection; `projectionBlockedCells` counts projection failures; `nativeGridBlockedCells` counts padding for stable placement. In `terrain-navigation.png`, green is reachable, orange blocks ground/building, and purple also blocks flight. It is computed pathing, not a game capture.

From the repository root:

```powershell
go -C golang test ./internal/converter/mapexporter/... ./internal/converter/wowmodel/direct/wmo ./internal/converter/wowmodel/direct/m2 ./internal/converter/common ./internal/server/api
```

With the desired build loaded in the running data server:

```powershell
go -C golang test -tags=integration_tests ./internal/converter/mapexporter/terrain -run '^TestStormwindCollisionTerrain$' -v -count=1 -timeout 25m
```

This exports map 0, cell 30,48 to `maps/navigation-stormwind.w3x`, reusing cached assets and placing NPC destructibles for inspection. `WOW_NAV_PRODUCT` optionally asserts the loaded product; `WOW_DATA_SERVER_SOCKET` selects a server. The test does not switch builds.

Saved maps can be checked without loading WoW data:

```powershell
$env:WOW_NAV_VERIFY_MAP='maps/navigation-stormwind.w3x'
go -C golang test ./internal/converter/mapexporter/terrain -run '^TestExportedNavigationBlockerFootprints$' -v -count=1

$env:WOW_NAV_NATIVE_MAP='maps/native-saved-copy.w3x'
go -C golang test ./internal/converter/mapexporter/terrain -run '^TestNativeEditorSavedBlockerRaster$' -v -count=1
```

Paths may be absolute or repository-relative. Tests decode resources, simulate native snapping/anchoring and compare footprint cells with WPM/PNG, including liquid pathing. The separate editor-save readback permits additional terrain/object pathing but requires predicted blockers to be present. Native movement, projection seams and editor round-trips remain separate validation.
