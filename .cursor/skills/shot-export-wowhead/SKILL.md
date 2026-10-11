---
name: shot-export-wowhead
description: Capture a Wowhead model-viewer canvas when a source screenshot or visual inspection is requested or needed.
---

# Capture Wowhead

Use installed Chrome, Edge, or Chromium. From the repo root:

```
go -C golang run ./cmd/shot-wowhead <url> --seq Stand --view front
```

The CLI adds `#modelviewer` when needed and prints labeled 1440×900 PNG paths containing only the canvas. Read them; blank frames fail.

Flags: `--seq Stand` (default; WoW name, first prefix match), `--variant 0` (later animation match), `--view` (front, left, back, right, top, bottom; comma-separated; omit for all six), `--out dir` (also accepted as second positional argument; default `tmp/shots`, relative to repo root). Files are `<slug>-<sequence>-<view>.png`.

For a comparison with a converter export, use [shot-export-wow-converter](../shot-export-wow-converter/SKILL.md), which owns fresh exports, mapped animation selection, and paired cameras. For standalone camera-map or sheet capture, read [camera options](references/cameras.md).

Standalone captures use model-bounds framing, freeze at the animation midpoint, isolate browser profiles, and wait for assets. Background: rgb(38,38,38); FOV: 45 degrees; lighting follows the converter's world-up curve.

`snapshot.ts` beside this file is a compatibility wrapper around `go run`; no screenshot executable needs rebuilding.
