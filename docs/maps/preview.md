# Map export preview

The dialog uses `golang/internal/converter/mapexporter/preview/` and `MapExportPreviewViewer`. It provides a source-data overview; production collision fitting, native water quantization and final placement can differ. See [map export](README.md) for output behavior.

## Dialog and selection state

Opening starts in compact mode. Expand fills the viewport without resetting the camera or reloading assets; Escape exits fullscreen first. Generate, X and Cancel preserve other form settings separately for each map/cell selection while the map page remains mounted. Texture limits and building interiors use the product preferences described in [map export](README.md).

Terrain clamps persist in the separate `wow-converter.map-clamps` localStorage key, shared across WoW products and builds. It retains the 100 most recently used areas, keyed by map ID and a sorted, deduplicated cell set. Opening a saved area restores its range and refreshes its LRU position. Slider edits, successful suggestions and export update the entry; least recently used areas are evicted when full. Invalid entries are ignored, and unavailable browser storage leaves in-memory editing functional.

The clamp slider enforces a selection-dependent minimum gap, rounded up to its 0.1 percentage-point step, to keep both terrain dimensions within 480 tiles. Generate waits for bounds; impossible selections require fewer cells. Bounds exclude vertices used only by terrain holes. Export validates dimensions again before allocating the height grid or deleting Fresh export output.

NPC-supported selections without saved clamps automatically apply Suggest only on their first opening, once availability is known. Restored clamps take priority over automatic suggestions and are still repaired to the current selection's minimum safe gap after preview bounds load. Manual edits cancel pending suggestions and take precedence; the Suggest button remains available to replace the remembered range. The dialog sends `autoClampPercent: false` to preserve the chosen range. NPC availability checks run alongside geometry loading. NPC-dependent controls are unavailable when the selection has no supported NPC data.

NPC markers are hidden when export is off. Otherwise green means a predicted unit placement, yellow a destructible, and red an excluded spawn. This prediction uses source floor distance, clamp and decoration mode; it omits M2 props and the full collision search. The header shows the output WC3 width × height using the exporter's four-cell rounding.

The faction panel appears only with predicted unit placements. It sends faction-template counts to `POST /api/maps/preview-factions`, which reuses the exporter's merged-player and overflow policy. Allies (teal), enemies (red), named neutral groups and generic neutral totals (gray) reflect NPC-to-user reactions for the selected Alliance/Horde player. Same-name entries within each reaction group combine their unit counts; each row includes a total. All faction counts stay white on a dark inset in either theme. Clamp, path blocker and decoration settings update counts without rebuilding geometry. Preview metadata includes faction-template IDs; the scene-cache revision invalidates older manifests while retaining reusable geometry. The map controls use theme-aware surfaces and contrasting text in both themes.

## Cached source data

`POST /api/maps/{map}/preview` caches ADT meshes sampled every two corner vertices, clustered WMO meshes, 512-pixel texture mipmaps, binary floor samples, masked liquids and scene manifests under `exported-assets-browse/_map-preview-v5/<build-hash>/`. Per-asset caches survive cell/map changes and normal server startup. Settings update the renderer without full model conversion, texture baking or NPC asset export. Missing/truncated assets rebuild; explicit browse-cache cleanup removes preview caches too. Scene metadata includes NPC database/name revisions while retaining reusable geometry.

`POST /api/maps/{map}/suggest-clamp` uses a separate ADT/NPC metadata manifest, skips WMO work and calls the same `SuggestTerrainClamp` as production export.

`preview?progress=1` streams NDJSON stages/counts followed by the result or error; ordinary requests return JSON. Closing the dialog or changing selection/build aborts the request and preparation, retaining completed per-asset caches. The renderer reports loaded/uploaded counts including cache hits. No detached job or polling is involved.

WMO preparation first counts groups across unique WMO models, then reports **Preparing WMO groups (N/M)** with a fixed total and cumulative completions. Cached groups count as finished, repeated placements do not inflate the total, and moving to another WMO does not reset the counter.

![Preview preparation progress](preview-wmo-progress.png)

## Display and picking

Flattened root ground is dirt-colored; removed root geometry stays at source height as wireframe. Child WMOs retain their geometry/materials. Only ADTs or explicitly marked roots are clamped; the current cell selector exports ADT roots.

Liquids reuse source surfaces and cached type classification. Blue water follows source depth/masks; orange guides indicate excluded clamp/height ranges. WMO water receives its instance transform once. Export rounds shores to native corners, so this display does not promise identical shoreline coverage.

Default/Fit looks north along WC3 +Y, with +X right. Axis conversion is already in preview positions; map angle is applied once to geometry, water and markers. Camera gestures match the model viewer; rendering runs on input, resize or settings changes.

Hover uses a lazy 1×1 GPU target for visible solid geometry and NPC spheres, at most once per animation frame and suspended while dragging. Wireframe and excluded-water guides are not pickable. Tooltips identify NPCs or clamp-based Playable/Unreachable areas; they do not report exact collision reachability.
