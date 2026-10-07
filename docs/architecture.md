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

## Trace a change before editing

Character exports enter `server/api/export_character.go`, run through the queue into `converter/character/`, convert M2/WMO data into an MDL graph, then write model/texture assets and return export metadata. Equipment and mounts reuse this pipeline. Map generation enters `server/api/maps_generate.go` and `converter/mapexporter/`; it also reuses model conversion. Search callers of any changed contract across both paths.

Keep domain structures typed in memory; serialize at file or transport boundaries. JSON round-trips are not a substitute for converting between Go types. Keep helpers in the owning package and document non-obvious invariants beside the implementation. New cross-module decisions belong in an ADR, not scattered comments.

Direct M2/WMO builders pass `converter/wowmodel/bundle/metadata.Data` to `File.LoadFromData`, which owns a typed assembly snapshot. Disk companion JSON enters through `Parse`. Preserve source/snapshot isolation and nullable animation timestamps; colors/texture transforms use `metadata.Track`, based on the bone animation contract, while emitters/cameras/lights use native M2 structures. Character customization choices share the dependency-free schema in `wow/character/meta/`.

Bundled `InProcessClient` calls local services or handlers directly. Its external unix socket is available to tools; the converter itself does not send every operation through that socket. An export failure must not reset the shared CASC runtime.

Queue handlers propagate `job.Context()`. A deadline or cancel request keeps the job processing and capacity occupied until its handler and started workers return. Synchronous native encoding, optimization, and serialization can finish their current call before stopping; late success cannot overwrite cancellation. See `server/util/jobqueue.go` and `converter/common/worker_pool.go` for lifecycle ownership.

## Choose checks by behavior

Commands run from the repository root unless stated otherwise. These are current entry points, not newly introduced verification tiers.

| Change or purpose | Check |
|---|---|
| Go logic or contracts | `go -C golang test ./internal/<changed-package>/...`; widen to `go -C golang test ./...` for shared contracts |
| Queue concurrency/lifecycle | `go -C golang test -race ./internal/server/util`; test affected API callers too |
| WoW-reader/conversion integration | Running converter/data server, then `bun run test:golang-integration`; narrow tagged Go tests with `-run` while iterating |
| Export appearance | Running converter, then selected snapshot cases and paired shots; broaden to `bun run test:snapshot` for shared visual behavior |
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
