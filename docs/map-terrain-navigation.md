# Collision-based map terrain

Map generation keeps the existing terrain clamp and map/object scale calculation, then fits WC3 ground to NPC-reachable collision surfaces. Navigation runs even when NPC model export is turned off.

**Place path blockers** is an opt-in checkbox, shown only when the selected tiles have supported NPC data. It defaults off and works independently of **Export NPCs**. The API accepts `placePathBlockers: true` to enable placement. With it off, terrain fitting and diagnostic PNG/JSON still run, but no blocker doodads or generated WPM are written; a cached WPM is removed. Generated blocker types enable the World Editor click helper (`duch = 1`).

`converter/mapexporter/terrain_navigation.go` rasterizes ADT triangle surfaces and the transformed collision meshes of placed WMO/M2 assets into 32-WC3-unit pathing cells. WMO faces use MOPY collision rules, including collision-only faces outside render batches. M2 uses its collision mesh rather than visual geometry. Direct converters retain that geometry only for bulk/map exports.

An NPC seeds the search when collision supports its feet within `seedSnap` and it can reach a nearby standing cell. Airborne or unsupported spawns do not seed it. This is physical grounding, not a creature-template flying/ground flag classification. Source NPCs come from bundled AzerothCore content; their locations can differ from newer client builds.

Server creatures and gameobjects use the base world with no active events. Positive-only `game_event_creature` / `game_event_gameobject` spawns are excluded from both placement and collision; negative event links remain present. This prevents Arena Tournament NPCs (Arena Organizer and TAR pedestals in Northshire) and inactive-event invisible barriers from appearing in the ordinary starting zone.

The multi-source breadth-first search retains separate vertical spans until projection. Only upward collision faces supply floors; ceiling undersides still obstruct movement. Walking follows sampled floor heights under the body radius, so probes on the uphill side of a sloped road start above its surface. Movement checks slope, steps, body radius/clearance, drops and collision along parabolic jumps. It is a static, sampled humanoid traversal approximation; it does not execute a WoW movement controller. Jump-only routes become walkable WC3 terrain, with short gaps filled by invisible ground ramps. Moving platforms, scripted doors and swimming are not simulated.

Where reachable surfaces overlap, the component supported by the most NPCs wins. Ties prefer shorter search distance, then the lower surface. Native corners evaluate the nearest selected collision plane in the winning component, including every corner supporting a narrow road. Copying cell-center heights to corners would introduce artificial steps. WC3 still has one native ground height per XY location. Cells that cannot match the selected surface after projection to 128-unit terrain corners are blocked instead of creating routes between floors. Wall footprints also block cells even when NPCs seed both sides.

This follows the same broad surface-based method as AzerothCore: `src/tools/mmaps_generator/MapBuilder.cpp` rasterizes solid collision into multiple spans, filters clearance/ledges and erodes the footprint by actor radius. `src/server/game/Movement/MovementGenerators/PathGenerator.cpp` advances with Detour `moveAlongSurface` and `getPolyHeight`. Its generator defaults to a 60-degree slope and approximately 1.6 WoW units of climb; the defaults below deliberately remain more conservative. Ordinary navmesh movement and explicit parabolic jumps are separate in AzerothCore; the exporter adds jump connections to satisfy map generation's reachability policy.

Unreachable cells block walking and building. Collision more than `airBlockHeight` above the nearest reachable surface also blocks flight. The exporter writes both `war3map.wpm` and custom **doodad** blocker types/instances (`war3map.w3d` and `war3map.doo`); blockers are never destructibles. Their model is `empty.mdl` from `resources/template-empty.w3x` (no geosets or nodes), not stock LOSBlocker.

Blockers use **solid square footprints only**. Packing compares four disjoint scan directions with a largest-first cover that allows overlap within the same blocking class, then keeps the cover with the fewest instances. Each square covers blocked cells only, has no holes and shares its type with other squares of the same size/class. The minimum square is 2x2 pathing cells (64x64 WC3 units); larger sizes are even multiples of pathing cells. Stock 2x2/4x4 footprints and ground-only 8x8 are reused; other sizes use uniformly colored BGR TGAs under `pathing/square-*.tga`. These are solid square pathing resources, not irregular bitmap masks. Packing is greedy, not a proof of a minimum cover. The exporter currently chooses axis-aligned squares from the allowed square/diamond shapes.

Blocker facing and fixed rotation use 270 degrees, cancelling the additional 90-degree pathing rotation. A native World Editor save demonstrated that centers snap to 64-unit increments, with a 32-unit offset when the integer half-size is odd. Odd footprints also anchor using `ceil(size/2)`, unlike the earlier reference blitter. The exporter therefore combines each 2x2 pathing-cell block conservatively (the strongest blocking class wins), packs on that 64-unit grid, and emits even-sized squares at stable centers. This prevents editor snapping from moving blockers into clear ground or leaving gaps under obstacles. Narrow edges can gain up to one pathing cell of blocking; `nativeGridBlockedCells` counts formerly open cells added by that adjustment. The PNG and WPM use the same adjusted grid.

Tests decode saved square resources, simulate the native snap/anchor rules and compare all cells with WPM/PNG. An optional native-readback test compares those predictions with a separate World Editor save; it permits additional terrain/other-object pathing but requires every predicted blocker to be present in the native WPM. Exact arbitrary 32-unit silhouettes cannot be represented by these native-stable solid squares without padded/masked resources, which map generation does not use.

## Configuration

The optional `navigation` object on `POST /api/maps/{map}/generate-wc3` overrides individual defaults. Distances are in source WoW units and follow the current export scale. `maxSlopeDegrees` is in degrees. Omitted properties retain defaults; explicit zero disables jumping or stepping where applicable.

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

Without grounded NPC seeds, generation logs the reason and retains the old ADT terrain behavior. `disabled: true` also selects that behavior. The selected height-clamp interval remains the native terrain budget; collision outside it cannot become playable ground.

## Diagnostics and validation

Generated maps include `terrain-navigation.json` with seed, coverage, packing and median NPC height-error metrics, and `terrain-navigation.png`. `traversalReachableCells` counts selected cells before native terrain fitting; `projectionBlockedCells` distinguishes projection failures from unreachable collision; `nativeGridBlockedCells` records additional cells blocked by the native placement-grid adjustment. Green means reachable ground, orange means ground/build blocked with flight allowed, purple means ground/build/flight blocked. The preview is a computed pathing diagram, not an in-game screenshot.

Focused tests:

```powershell
go -C golang test ./internal/converter/mapexporter ./internal/converter/wowmodel/direct/wmo ./internal/converter/wowmodel/direct/m2 ./internal/converter/common ./internal/server/api
```

With the data server already running and the desired client build loaded:

```powershell
go -C golang test -tags=integration_tests ./internal/converter/mapexporter -run '^TestStormwindCollisionTerrain$' -v -count=1 -timeout 25m
```

To check a saved map without loading WoW data:

```powershell
$env:WOW_NAV_VERIFY_MAP='maps/navigation-stormwind.w3x'
go -C golang test ./internal/converter/mapexporter -run '^TestExportedNavigationBlockerFootprints$' -v -count=1
```

`WOW_NAV_VERIFY_MAP` accepts an absolute map directory or a path relative to the repository root.

The integration test exports map 0, ADT tile 30,48 into `maps/navigation-stormwind.w3x`, reusing existing assets (`FreshExport` is false). It uses the existing data server without switching builds. `WOW_NAV_PRODUCT` optionally asserts the expected product; `WOW_DATA_SERVER_SOCKET` can select an isolated server. NPCs export as actual doodads at their source elevations, without pathing footprints, for inspection against the generated ground. Source geometry, computed terrain and serialized blocker coverage are offline evidence. Ground/air movement, projection seams, solid square doodad footprints and editor save/reload still require verification in Warcraft III.



To validate footprint rasterization against a map saved separately through World Editor:

```powershell
$env:WOW_NAV_NATIVE_MAP='maps/native-saved-copy.w3x'
go -C golang test ./internal/converter/mapexporter -run '^TestNativeEditorSavedBlockerRaster$' -v -count=1
```
