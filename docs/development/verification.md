# Verification

Run the commands from the repository root unless stated otherwise.

| Change or purpose | Check |
|---|---|
| Unit tests for Go, the web UI, shared tools, and the Wowhead sampler | `bun run test:unit` |
| Go logic or contracts | `go -C golang test ./internal/<changed-package>/...`; widen to `go -C golang test ./...` for shared contracts |
| Queue concurrency/lifecycle | `go -C golang test -race ./internal/server/util`; test affected API callers too |
| WoW-reader/conversion integration | Running converter/data server, then `bun run test:golang-integration`; narrow tagged Go tests with `-run` while iterating |
| Export appearance | Running converter, then selected snapshot cases and paired shots; broaden to `bun run test:snapshot` for shared visual behavior |
| M2 effects without Wowhead support | Embedded original renderer and warmed export pairs: [WoW source shots](../rendering/source-viewer.md) |
| API contract | Running converter, then `bun run test:api` |
| Map export | Running converter, then `bun run test:regression-maps` for asset-import/file-presence smoke coverage; verify terrain and gameplay in WC3 separately |
| Release package | `bun run verify` runs unit tests, builds and boots the bundle, then runs Go integration, API tests, and snapshots. |

To exercise the live Lich King screenshot comparison, set `WOW_CONVERTER_CAPTURE_TEST_URL` to an isolated converter with WoW data loaded and run `go -C golang test ./internal/server/reportshot -run '^TestLocalLichKingCapture$' -v -count=1`. The test exports to `capture-tests/the-lich-king` and captures the matching Stand sequence in the converter and on Wowhead.

For a selected snapshot in PowerShell:

```powershell
$env:SNAPSHOT_SUITE='retail'
$env:SNAPSHOT_SLUG='<slug>'
bun test tests/snapshot-tests/model/model.test.ts
Remove-Item Env:SNAPSHOT_SUITE, Env:SNAPSHOT_SLUG
```

Full snapshots switch Retail/Classic products and restore the original product. Retained exports use separate Retail/mount and Classic namespaces so shared texture paths cannot overwrite another product. They require loaded WoW data and an installed supported browser. Go integration discovery includes expensive texture-baking tests and the WMO profiling test; it is not a quick unit gate. `verify` builds/boots its own app and clears snapshot filters so it runs the whole catalog.

Map regression is currently disabled in `scripts/verify.ts`. Browser snapshots protect appearance regressions; they do not certify Warcraft III combat stats or every in-game material/animation effect. Preserve approved baselines during refactors, investigate differences, and follow the [snapshot rule](../../.cursor/rules/snapshot-tests.mdc) and [mismatch skill](../../.cursor/skills/debug-wowhead-mismatch/SKILL.md) for visual evidence and intentional updates.

Report what changed, the exact checks run, and unverified behavior. Performance claims need comparable inputs/configuration and measured durations or memory; a shorter command alone is not evidence of faster conversion.
