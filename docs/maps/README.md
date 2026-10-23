# Map export

Map generation produces unpacked `.w3x` directories under `maps/`. Start the Go app with `bun run dev`, select map cells, and open **Generate WC3 map**. The dialog controls rotation, height clamp, unit scale, building interiors, NPC export, path blockers, terrain mode and Fresh export.

| Topic | Guide |
|---|---|
| Dialog, clamp suggestions and cached preview | [Preview](preview.md) |
| Native ground, stock tile selection and model boundaries | [WC3 tilesets](terrain-tilesets.md) |
| Collision fitting and structural blockers | [Navigation](terrain-navigation.md) |
| Native water and liquid pathing | [Water](water.md) |
| Removed ground-foliage experiment and findings | [Ground foliage](foliage.md) |
| NPC players, fields, models, voices, death audio and icons | [NPC export](npcs.md) |
| Reproduce terrain comparisons and evaluate quality | [Terrain benchmark](terrain-benchmark.md) |

**Terrain as WC3 tilesets** defaults off. Whole-ADT model terrain uses converted models for the ground; native mode uses editable WC3 ground inside the chosen height window and models outside it. **Export NPCs** defaults off. **Place path blockers** defaults on in the UI when NPC data is supported, independently of NPC export; direct Go/API configuration defaults it off.

## Textures

The **Textures** section above **Fresh export** controls three independent resolutions:

| Setting | Choices | Default |
|---|---|---|
| Terrain | 1k, 2k, 4k, 8k | 4k |
| Unit | 256px, 512px, 1024px, Unlimited | Classic: 256px; retail: Unlimited |
| Doodad | 256px, 512px, 1024px, Unlimited | Classic: 256px; retail: Unlimited |

All three and **Include building interiors** are saved together in browser localStorage under `wow-converter.map-textures.<product>`, independently for products such as `wow` and `wow_classic`, and survive build updates. Interiors default on; older saved texture settings retain their values. Terrain sets the ADT bake resolution in either terrain mode. Unit caps NPC textures, including NPC destructibles; Doodad caps other scenery while preserving aspect ratio. Changing limits refreshes affected cached textures without Fresh export. Terrain textures retain their own resolution.

![Map texture settings](npc-texture-size.png)

## Template and startup

`resources/template-empty.w3x` supplies map defaults and custom Lua triggers. New exports copy it; existing output receives missing template files. Every generation refreshes template Lua/WCT/WTG and inherits the alpha-tile minimap color and loading-screen settings in W3I. Template WTS loading-screen strings are resolved without replacing the output's other strings or generated bounds/placements.

Selected ADT minimaps are stitched into a rotated, bounds-aligned 256×256 `minimap.blp`; gaps and padding are black. This import is separate from World Editor's `war3mapMap.blp`.

Both modes select `minimap.blp`. Model terrain additionally hides native terrain, disables fog/fog mask, and preloads ADT chunks behind a black screen in cinematic mode by moving the camera to their injected positions. Cinematic mode hides the interface and disables player control during preload, then restores them before fading back in. Root WMOs are used when no ADT has geometry. Native tileset mode leaves terrain and vision enabled and empties the preload action, including its fade/camera movement. Startup changes are injected into both runtime Lua and custom-trigger WCT so they survive editor saves. Imports share `wc3/extra.EnsureCustomImports`.

ADT visuals use a 4×4 grid of independently placed models per source tile; empty parts are omitted. Change `adtChunksPerSide` in `converter/mapexporter/terrain/adt_chunks.go` to choose another grid (8 means 64 models). Each part is centered on its own XYZ bounding box, with its placement offset preserving the original geometry. Model, sequence and geoset bounds enclose the centered vertices; the generic large-model bounds override is bypassed. Full-model terrain assets depend on source geometry and split size, not height clamps; clamp changes affect map terrain and instance transforms. Tileset fallback models are split after clipping and boundary welding, so their assets still depend on the fitted map.

Final encoded terrain BLPs are deduplicated by content, including mipmaps, with model references remapped to a stable shared filename. Resolution, palette and alpha data stay intact. After successful map serialization, obsolete converter-owned terrain models/textures and their import entries are removed from this map's terrain directory. Custom filenames and other asset directories are preserved. Unchanged chunk MDX files are reused without rewriting them.

Unit placements get unique creation numbers and the native v8/11 layout with Reforged skins. Lua creates serialized units after `InitGlobals`, before initialization triggers. WCT omits this generated creation code because World Editor rebuilds it from placements. Re-exporting replaces old creation calls, including when NPC export is disabled.

## Gameobjects and scenery

**Include building interiors** controls WMO child props, not building geometry. Exterior decorations are always exported from the active doodad sets. Unchecking the option omits only placements whose group `MODR` references are exclusively from interior groups (`MOGP` flag `0x2000`, without exterior flag `0x8`). Children shared with exterior or unclassified groups, and children with no group references, are kept. Missing group data keeps all children for that building and emits a warning. Classification applies per placement, so the same model may be kept outside and omitted inside. Regenerate the map after changing this option.

Client placements combine with bundled AzerothCore `gameobject` spawns/templates. Display IDs resolve through client `GameObjectDisplayInfo`; server orientation includes quaternion exceptions and addon overrides. Gameobjects become destructibles in W3B/DOO, with a `gameobjects.json` source/type manifest. Missing displays, model-less entries and unavailable assets are logged/skipped; database failures and cancellation stop generation. Encounter scripts and initial animation states are not translated.

Server NPCs and gameobjects use the base world with no active events: positive-only event spawns are omitted, negative event links remain, and multiple links do not duplicate a spawn. This policy applies to collision as well as placement.

ADT doodad types use Ignore Model Click (`dimc=1`); direct doodad children use Occlusion Auto (`doct`), excluding units/gameobjects. Without ADTs, root WMOs receive `dimc`. Shared models can use separate object types when editor fields differ by placement.

Doodads and gameobjects store pitch/roll per instance in WC3 3.0 DOO v12, preserving small tilts without duplicate rotation types. Source `Rz*Ry*Rx` rotations are converted to WC3's `Rz*Rx*Ry`; type-level max pitch/roll are zero. Regeneration removes old rotation variants while reusing cached assets. The conversion follows the [Doodad Handler orientation reference](https://www.hiveworkshop.com/threads/doodad-handler-preplaced-special-effect.374394/).

## Code ownership and verification

All owners below are under `golang/internal/`:

| Owner | Responsibility |
|---|---|
| `server/api/maps_generate.go` | Request validation and export jobs |
| `converter/mapexporter/` | Run, template, startup and map-file orchestration |
| `converter/mapexporter/creature/` | NPC placement, ownership, fields and assets |
| `converter/mapexporter/terrain/` | Native ground, tilesets, boundaries, navigation and water |
| `converter/mapexporter/preview/` | Fast source-data preview |
| `converter/mapexporter/settings/` | Shared configuration; root aliases retain CLI/API callers |

Run `go -C golang test ./internal/converter/mapexporter/... ./internal/server/api ./internal/wc3/extra` for focused coverage. See [verification](../development/verification.md) for broader checks. Serialization/browser tests do not establish native rendering or gameplay correctness; use a fresh WC3 session for object-data changes and test World Editor save/reload separately.
