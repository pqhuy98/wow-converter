---
name: shot-export-wow-converter
description: Screenshot an exported Warcraft 3 model from the local wow-converter viewer. Use when checking an export visually, verifying a model fix, when the user asks for a shot of an exported MDX, or when a wowhead.com URL should be compared with the converted model.
---

# Shot export (wow-converter)

The converter must already be serving http://127.0.0.1:3001. Use the installed Chrome, Edge, or Chromium.

From the repository root (`bun run shot` is the same CLI):

```
go -C golang run ./cmd/shot-converter <asset-path> --seq "Stand 1" --view front
```

The command runs the same Go capture code as the report server. It prints one labeled 1440×900 PNG path per selected view. Read the PNGs; blank frames fail. `<asset-path>` is relative to `exported-assets`; a full path containing that directory also works.

Flags: `--seq Stand` (default, prefix matches `Stand 1`), `--view front` (omit for all six; comma-separated views also work), `--out tmp/shots` (default; relative dirs are repo-root), `--base http://127.0.0.1:3001` (or `WOW_CONVERTER_URL`). `--cameras <Wowhead map.json>` applies saved source cameras; `--model-scale <export metadata modelScale>` overrides their units for a differently sized export. Standalone files are `<model>-<matched-sequence>-<view>.png`.

For a Wowhead comparison, freshly export the model first, then capture both sources in one command:

```
go -C golang run ./cmd/shot-converter <asset-path> --seq "Stand 1" --wowhead <url> --wow-seq Stand --view front --out tmp/compare-shots
```

This captures only front for both sources. The converter supplies its preliminary silhouette aim; Wowhead applies the report server's side refinement, then its actual eye/target/up is transferred into converter model coordinates. Top/bottom use Wowhead's fixed cameras. Paired filenames end in `-wowhead.png` or `-converter.png`. Read both images.

Fresh export: POST `/api/export/character` with `character.base` `{"type":"wowhead","value":"<url>"}`, `character.inGameMovespeed` 270, `outputFileName` set to the page slug, `optimization` `{}`, and `format` `mdx`. Poll `/api/export/character/status/{id}` until `done`; fail on `failed` or `cancelled`. Use the returned `result.exportedModels[].path` and `result.reportMetadata.models[].sequences[]` for the WC3 `name`, WoW `wowName`, and `wowVariant` (`--variant`). Never guess the source animation from a WC3 name. Runtime table: `golang/internal/converter/wowmodel/animation/` (generated from `src/lib/converter/wow-model/animation/animation-mapper.ts`).

Shot mode deterministically freezes the parent, attachments, and particles at the animation midpoint. Browser profiles are isolated, HTTP caching is disabled, and the development overlay is hidden. Background is rgb(38,38,38); FOV is 45 degrees. Views are front, left, back, right, top, bottom. Standalone shots retain local mesh framing.

The Lich King report check always exports before shooting: `go -C golang test ./internal/server/reportshot -run TestLocalLichKingCapture -v -count=1`. The converter must already be running. The broader visual regression suite remains `bun run test:snapshot`; cases are `tests/snapshot-tests/model/<suite>/<slug>/`. Update a case with `SNAPSHOT_UPDATE=1 SNAPSHOT_SUITE=retail SNAPSHOT_SLUG=<slug> bun test tests/snapshot-tests/model/model.test.ts`, then inspect its actual/diff/expected PNGs.
