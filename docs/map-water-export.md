# Map water export

Map generation automatically includes source liquids. It does not change the selected map scale or terrain clamp. NPC export and the optional **Place path blockers** checkbox are independent of water export.

The Go pipeline carries masked ADT MH2O/legacy MCLQ cells and WMO MLIQ cells as typed geometry through the existing conversion snapshots and object tree. ADT liquid elevations are absolute WoW Z; WMO elevations are local and receive the complete placement transform once. Terrain snapshots release their liquid payload after the parsed model owns a copy. Reading these source chunks does not require a fresh model/texture export.

`mapexporter/terrain_water.go` applies the final terrain transform, including map rotation and clamp. It writes ordinary water/ocean to native WC3 terrain corners while preserving sloping rivers and the bed beneath them. Source liquid depth participates in the NPC ground search. A selected bridge or upper floor above a liquid remains dry.

Water rendering uses native WC3 water only, with no liquid surface doodad. WC3 has one surface per XY corner and a 128-unit grid. Projection keeps wet source samples covered and supplies coherent neighboring heights. Small pools and shorelines round to this grid; strict mask clipping would create dry strips inside narrow rivers. Source chunk origin seams smaller than 0.0625 WC3 units are joined. Source masks still determine liquid movement flags.

Stacked levels and source slopes that cannot fit the native grid are recorded as `approximatedCells`. Native height overflow, magma/slime and unknown types are recorded under `unsupportedReasons`; no extra mesh or scale change is introduced. Ordinary water uses the generated map's WC3 water appearance. This cannot reproduce every WoW liquid material, overlapping surface or waterfall.

`LiquidType.db2` supplies water/ocean/magma/slime classification. Known legacy IDs remain available if that table cannot be read. Unknown IDs are preserved and reported.

## WC3 files

- `war3map.w3e`: version 12 water presence is bit `0x100`; semantic flags use the older positions in memory. Water height uses the low 14 bits, independently of the boundary marker `0x4000`. For the generated Lordaeron tileset, visible Z is `(raw - 8192) / 4 - 89.6`, with 0.25-unit precision and range −2137.6…1958.15.
- `war3map.wpm`: source water gets swimming (`64`) and no-building (`8`) flags. Depth greater than 51.2 WC3 units also blocks ordinary walkers (`2`). Liquid itself never adds no-flying (`4`). Physical obstacle flags are merged when blocker placement is enabled. This file remains present for liquid movement even with NPCs or blocker doodads disabled.
- Cached exports remove the former generated `water/liquid-*` model/texture files. New map object data contains no liquid surface doodad.
- `terrain-water.json`: source layers/cells/types, native corner count, approximated/unsupported cells, height range and shallow/deep pathing counts.
- `terrain-water.png`: source water pathing preview; light blue is shallow, dark blue is deep, orange is non-water liquid. It is separate from the structural navigation preview.

## Checks and native limits

Run `go -C golang test ./internal/wc3/translators ./internal/wow/formats/adt ./internal/wow/formats/wmo ./internal/converter/common ./internal/converter/mapexporter`. Fixtures cover independent v11/v12 terrain bytes, masks, absolute/local heights, slope preservation, snapshot ownership and rotation, dry bridges, deep water, small native pools, unsupported layers/materials, continuous narrow rivers/chunk seams, cached mesh cleanup, and water with NPC/blocker export disabled.

To check saved structural blockers together with water pathing, set `WOW_NAV_VERIFY_MAP` to the generated map directory and run `go -C golang test ./internal/converter/mapexporter -run TestExportedNavigationBlockerFootprints -count=1`.

Source/serialization checks do not prove native rendering, World Editor save stability, or engine movement. Native shorelines, walking/swimming/flight and bridge traversal still require an editor/game check. The initial depth threshold follows HiveWE's implementation and may require adjustment against a native save.
