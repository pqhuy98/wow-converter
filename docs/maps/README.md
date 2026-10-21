# Map generation

Map generation enters `golang/internal/server/api/maps_generate.go` and `golang/internal/converter/mapexporter/`. The Go path loads selected ADTs into a typed in-memory registry and reuses the production model converter. Generated maps are unpacked `.w3x` directories under `maps/`.

The export dialog controls the selected cells, rotation, terrain height clamp, building interiors, NPCs, path blockers and Fresh export. [Preview](preview.md) explains the approximate scene, settings retention and one-time automatic clamp suggestion. [Terrain navigation](terrain-navigation.md) and [water](water.md) describe the native terrain/pathing limits.

## Template, minimap and startup

`resources/template-empty.w3x` supplies map defaults and custom Lua triggers. New exports copy it; existing output receives missing template files before generated map data is saved. Every generation refreshes template Lua/WCT/WTG startup data and injects current placement coordinates.

Selected ADT minimap tiles are stitched into a 256×256 `minimap.blp`, using final map bounds and rotation. Gaps and padding are black. The imported custom texture is separate from the editor-generated `war3mapMap.blp`, so an editor minimap rebuild does not replace it.

The Lua template selects `minimap.blp` and sweeps the camera behind a black screen over root ADTs with geometry, or root WMOs when there are no such ADTs. Export writes locations into `war3map.lua` and length-prefixed custom trigger text in `war3map.wct`, refreshes `war3map.wtg`, registers the texture in `war3map.imp`, and selects Lua in `war3map.w3i`. It uses template hooks directly. Camera rendering and preload effectiveness need verification in the game.

Unit placements receive unique creation numbers. New unit files use the native v8/11 layout with Reforged skins; saving converter v9 files repairs the unsupported version header. Generated Lua creates serialized units after `InitGlobals` and before initialization triggers. This creation code is kept out of WCT because World Editor rebuilds it from the placement file on save. Cached exports replace old creation calls, including when NPC export is disabled.

## NPC placements

NPC unit mode excludes positions outside the playable clamp or too far from fitted ground, while retaining those creatures in model export. At the recommended clamp width the ground tolerance is 100 WC3 units. Manual widths use `100 * recommendedWidth / chosenWidth`, so merely changing the clamp does not change a supported NPC's source-space floor status.

**All NPCs as destructibles** places decoration at source heights without pathing footprints. The retained API/config key is `allAreDoodads`. Preview spheres are green for units, yellow for destructibles and red for excluded placements. See [NPC export](npcs.md) for appearance sharing, animation policy, fields and icons.

## Gameobjects

Client placements combine with AzerothCore `gameobject` / `gameobject_template` spawns. Display IDs resolve through the loaded client's `GameObjectDisplayInfo`. Server placements follow AzerothCore orientation rules, with quaternion exceptions and `gameobject_addon` overrides.

Gameobjects become WC3 destructibles in `war3map.w3b`, with imported model filenames and individual placements in `war3map.doo`. `gameobjects.json` records source identities and WC3 type codes. Missing display rows, model-less displays and unavailable assets are logged and skipped; database failures and cancellation stop generation. Encounter scripts and initial animation states are not translated to WC3 triggers.

Server creatures and gameobjects use the base world with no active events. Positive-only event spawns are omitted; negative event links remain. Multiple event links never duplicate a spawn.

ADT doodad types use Ignore Model Click (`dimc = 1`), and direct doodad children use Occlusion = Auto (`doct`), excluding units/gameobjects. Without ADTs, only root WMOs receive `dimc`. Shared models use separate object types when these fields differ by placement.

## Verification

Package checks and map smoke tests are in [verification](../development/verification.md). Generated reports explain source calculations and exclusions; source/serialization tests do not certify terrain traversal, startup camera rendering or combat behavior in Warcraft III. Object-data changes require a restarted map session.
