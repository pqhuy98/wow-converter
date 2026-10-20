# Architecture and verification

Go owns production conversion and the offline desktop app. The Next.js UI is shared with the compatibility TS runtime; Go and TS need not have identical internals or features ([ADR 0001](decisions/0001-go-ts-pipeline-divergence.md)). Prefer changes in the owning module over another conversion implementation.

## Owners

All Go paths below are under `golang/internal/`.

| Owner | Responsibility |
|---|---|
| `server/api/` | Request validation, job submission/status, and response encoding |
| `server/util/jobqueue.go` | Job state, deadlines, cancellation, and concurrency accounting |
| `wow/client/`, `wow/service/` | Data access boundary and bundled/HTTP operation |
| `wow/formats/`, `wow/export/` | Read WoW formats and construct source export data |
| `converter/character/` | Character, creature, equipment, and mount orchestration |
| `converter/wowmodel/direct/m2/`, `converter/wowmodel/direct/wmo/` | Direct model conversion; shared texture baking currently lives in M2 |
| `converter/wowmodel/assemble/` | Assemble converted geometry, animation, materials, and metadata |
| `converter/common/`, `converter/texturesource/`, `converter/runtimecache/` | Asset output, texture sources, and runtime cache lifecycle |
| `converter/mapexporter/`, `azerothcore/` | Terrain/placement orchestration, source gameplay data, and WC3 map generation |
| `formats/mdl/`, `formats/mdx/` | WC3 model graph and MDL/MDX serialization |
| `wc3/`, `war3/` | Map types/serialization and generated object field definitions |

`golang/cmd/wow-converter/main.go` wires the bundled runtime and API. `webui/` owns UI behavior, `scripts/` owns build/dev orchestration, and `tests/` owns API/snapshot/map acceptance checks.

Map export stitches loaded ADT minimap tiles into `minimap.blp`, using final placement bounds/rotation and black gaps/padding; `war3mapMap.blp` stays editor-owned. The Lua template initializes that texture and sweeps the camera over placed root ADTs (root WMOs if no ADT has geometry) behind a black screen. Export injects final WC3 coordinates into both `war3map.lua` and length-prefixed `war3map.wct`, refreshes template `war3map.wtg`, registers the texture in `war3map.imp`, and selects Lua in `war3map.w3i`, so editor saves retain the startup behavior. Camera rendering/preload effectiveness still needs in-game verification.

`MapManager.Save` gives unit placements unique creation numbers, preserving existing valid IDs and repairing duplicates. New unit files use the native v8/11 layout with Reforged skins; saving cached converter v9 files repairs their unsupported version header. Map export also generates Lua unit creation from the serialized placements, called after `InitGlobals` and before initialization triggers. This generated code stays out of WCT: World Editor rebuilds unit creation from the DOO when saving, so a persistent custom creation trigger would duplicate units. Cached exports replace old creation calls, including when NPC export is disabled.

Map gameobjects combine client placements with AzerothCore `gameobject` / `gameobject_template` spawns from the bundled SQLite. Their display IDs resolve through the loaded client's `GameObjectDisplayInfo`; server placements follow `GameObject::Create` (orientation by default, quaternion for its explicit exceptions and `gameobject_addon` overrides). Gameobjects become WC3 destructibles (`war3map.w3b`) with explicit imported model filenames and retain individual placements in `war3map.doo`. `gameobjects.json` records exported source identities and WC3 type codes. Missing display rows, model-less displays, and unavailable model assets are logged and skipped; table/database failures and cancellation still stop generation. Instance encounter scripts and initial animation states are not translated into WC3 triggers.

Creature and gameobject export use the base world with no active events: positive-only `game_event_creature` / `game_event_gameobject` spawns are omitted, while negative event links remain present. This excludes Arena Tournament NPCs and temporary invisible event barriers from placement. Multiple event links never duplicate a spawn.

Regenerate all bundled tables with `bun run generate:acore-sqlite`. To update selected tables from AzerothCore's base SQL dumps while retaining other data, use `bun scripts/generate-azerothcore-sqlite.ts --sql-dir=<db_world-directory> --tables=gameobject,gameobject_template,gameobject_addon,game_event_gameobject,game_event_creature`.

## Trace a change before editing

Character exports enter `server/api/export_character.go`, run through the queue into `converter/character/`, convert M2/WMO data into an MDL graph, then write model/texture assets and return export metadata. Equipment and mounts reuse this pipeline. Map generation enters `server/api/maps_generate.go` and `converter/mapexporter/`; it also reuses model conversion. Search callers of any changed contract across both paths.

Keep domain structures typed in memory; serialize at file or transport boundaries. JSON round-trips are not a substitute for converting between Go types. Keep helpers in the owning package and document non-obvious invariants beside the implementation. New cross-module decisions belong in an ADR, not scattered comments.

Direct M2/WMO builders pass `converter/wowmodel/bundle/metadata.Data` to `File.LoadFromData`, which owns a typed assembly snapshot. Disk companion JSON enters through `Parse`. Preserve source/snapshot isolation and nullable animation timestamps; colors/texture transforms use `metadata.Track`, based on the bone animation contract, while emitters/cameras/lights use native M2 structures. Character customization choices share the dependency-free schema in `wow/character/meta/`.

Bundled `InProcessClient` calls local services or handlers directly. Its external unix socket is available to tools; the converter itself does not send every operation through that socket. An export failure must not reset the shared CASC runtime.

Queue handlers propagate `job.Context()`. A deadline or cancel request keeps the job processing and capacity occupied until its handler and started workers return. Synchronous native encoding, optimization, and serialization can finish their current call before stopping; late success cannot overwrite cancellation. See `server/util/jobqueue.go` and `converter/common/worker_pool.go` for lifecycle ownership.

## Animation export edits

Character export requests accept `character.sequenceSelection` (source record indices scoped to a model/build) and `animationRenames`: a list of `{wowName, wowVariant, newWc3Name}`. Renames apply to the base output after sorting and generated-animation processing, preserving sequence order. Overridden names serialize exactly, without a numeric suffix; names are limited to 79 UTF-8 bytes and cannot contain quotes, backslashes, line breaks or NUL. Conflicting renames for source records merged into one combined sequence fail explicitly.

Animation-only re-export passes the current output's `modelScale` to preserve scale. Export Character uses the current form settings and offers Keep changes or Discard for non-default animation edits when the base reference and build match the last successful export. Keep changes carries selection and renames, including drafts, without retaining the old scale override.

## Choose checks by behavior

Commands run from the repository root unless stated otherwise. These are current entry points, not newly introduced verification tiers.

| Change or purpose | Check |
|---|---|
| Go logic or contracts | `go -C golang test ./internal/<changed-package>/...`; widen to `go -C golang test ./...` for shared contracts |
| Queue concurrency/lifecycle | `go -C golang test -race ./internal/server/util`; test affected API callers too |
| WoW-reader/conversion integration | Running converter/data server, then `bun run test:golang-integration`; narrow tagged Go tests with `-run` while iterating |
| Export appearance | Running converter, then selected snapshot cases and paired shots; broaden to `bun run test:snapshot` for shared visual behavior |
| M2 effects without Wowhead support | Embedded original renderer and warmed export pairs: [WoW source shots](wow-source-shots.md) |
| API contract | Running converter, then `bun run test:api` |
| Map export | Running converter, then `bun run test:regression-maps` for asset-import/file-presence smoke coverage; verify terrain and gameplay in WC3 separately |
| Release package | `bun run verify`: Go units → UI/desktop build → boot bundled app → Go integration → API → full snapshots |

For a selected snapshot in PowerShell:

```powershell
$env:SNAPSHOT_SUITE='retail'
$env:SNAPSHOT_SLUG='<slug>'
bun test tests/snapshot-tests/model/model.test.ts
Remove-Item Env:SNAPSHOT_SUITE, Env:SNAPSHOT_SLUG
```

Full snapshots switch Retail/Classic products, clear exported assets, and restore the original product. They require loaded WoW data and an installed supported browser. Go integration discovery includes expensive texture-baking tests and the WMO profiling test; it is not a quick unit gate. `verify` builds/boots its own app and clears snapshot filters so it runs the whole catalog.

Map regression is currently disabled in `scripts/verify.ts`. Browser snapshots protect appearance regressions; they do not certify Warcraft III combat stats or every in-game material/animation effect. Preserve approved baselines during refactors, investigate differences, and follow the [snapshot rule](../.cursor/rules/snapshot-tests.mdc) and [mismatch skill](../.cursor/skills/debug-wowhead-mismatch/SKILL.md) for visual evidence and intentional updates.

Report what changed, the exact checks run, and unverified behavior. Performance claims need comparable inputs/configuration and measured durations or memory; a shorter command alone is not evidence of faster conversion.

Map terrain reachability is owned by converter/mapexporter/terrain_navigation.go and terrain_blockers.go. It uses source ADT/WMO/M2 collision, NPC seeds and packed doodad pathing footprints; see [collision-based terrain](map-terrain-navigation.md) for configuration, diagnostics and native verification boundaries.

Map doodad editor fields follow the source hierarchy: ADTs get Ignore Model Click (`dimc = 1`), and their direct doodad children get Occlusion = Auto (`doct`, string `Auto`), excluding units and gameobjects. Without any ADT, only root WMOs get `dimc = 1`. Shared models use separate object types when these fields differ between placements.

Map liquids travel through typed ADT/WMO model metadata into `mapexporter/terrain_water.go`. Water uses native terrain heights and source-derived WPM movement flags, with no liquid surface doodads. Shoreline detail rounds to the native grid, and unsupported/approximated surfaces are reported. Water pathing is independent of optional blocker doodads and NPC export. See [map water export](map-water-export.md) for encoding, diagnostics and native verification limits.
