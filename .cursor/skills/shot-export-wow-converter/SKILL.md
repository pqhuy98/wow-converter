---
name: shot-export-wow-converter
description: Capture an exported WC3 model, including paired Wowhead comparisons, when visual verification is requested or needed.
---

# Capture an export

The converter must be running at `http://127.0.0.1:3001` (or `--base` / `WOW_CONVERTER_URL`). Use installed Chrome, Edge, or Chromium. Commands run from the repo root; `bun run shot` is the same CLI.

```
go -C golang run ./cmd/shot-converter <asset-path> --seq "Stand 1" --view front
```

`<asset-path>` is relative to `exported-assets`; a full path containing that directory also works. The CLI prints labeled 1440×900 PNG paths. Read them; blank frames fail.

Flags: `--seq Stand` defaults to the first prefix match; `--view` accepts front, left, back, right, top, bottom (comma-separated; omit for all six); `--out` defaults to `tmp/shots`, relative to the repo root. Standalone shots use local mesh framing and filenames `<model>-<matched-sequence>-<view>.png`.

## Fresh export and comparison

POST `/api/export/character` with `character.base` `{"type":"wowhead","value":"<url>"}`, `character.inGameMovespeed` 270, `outputFileName` set to the page slug, `optimization` `{}`, and `format` `mdx`. Poll `/api/export/character/status/{id}` until `done`; fail on `failed` or `cancelled`.

Use `result.exportedModels[].path` and `result.reportMetadata.models[].sequences[]`: WC3 `name` maps to WoW `wowName` and `wowVariant` (`--variant`). Never guess a WoW animation from a WC3 name. Runtime mapping: `golang/internal/converter/wowmodel/animation/`, generated from `src/lib/converter/wow-model/animation/animation-mapper.ts`.

```
go -C golang run ./cmd/shot-converter <asset-path> --seq "Stand 1" --wowhead <url> --wow-seq Stand --view front --out tmp/compare-shots
```

The pair transfers refined Wowhead cameras into converter coordinates; top/bottom use fixed source cameras. Filenames end in `-wowhead.png` and `-converter.png`. Read both. Saved cameras use `--cameras <map.json>`; resized exports use fresh `--model-scale <reportMetadata.models[].modelScale>`, never scale inferred from mesh bounds.

Captures freeze animation, attachments, and particles at the midpoint, isolate browser profiles, disable HTTP caching, and hide the dev overlay. Background: rgb(38,38,38); FOV: 45 degrees.

For regression/baseline work, read [snapshot-tests](../../rules/snapshot-tests.mdc).
