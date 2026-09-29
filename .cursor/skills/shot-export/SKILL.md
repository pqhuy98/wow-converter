---
name: shot-export
description: Screenshot an exported Warcraft 3 model from the local wow-converter viewer. Use when checking an export visually, verifying a model fix, or the user asks for a shot of an exported MDX.
---

# Shot export

The converter must already be serving http://127.0.0.1:3001.

From the wow-converter repo:

```
bun scripts/shot-export.ts <asset-path>
```

`<asset-path>` is the path under `exported-assets`, such as `the-lich-king.mdx`. A full path that contains `exported-assets` is accepted.

Optional flags: `--seq Attack` (default Stand), `--view front` (default is all six: front, back, left, right, top, bottom), `--out dir`, `--base http://127.0.0.1:3001`.

The script prints one absolute PNG path per view. Read those images. A blank or flat frame means the shot failed.

Shot mode freezes the clock. The sequence plays from its start to the midpoint at a fixed 60Hz step, particles use a fixed random seed, then time stops. Two shots of the same file on this machine should match.

Views, with the model facing +X: front looks at the face, back looks from behind toward the front, left looks from the model's left toward its right, right is the opposite, top looks down, bottom looks up.

The regression suite shoots the same frozen viewer. `bun run test:visual` exports every retail and classic catalog case and compares a 2×3 sheet to `tests/visual/<suite>/<slug>/<slug>.expected.png`. `bun run test:visual:update` refreshes every ground truth; add `-- --slug=<slug>` for one case. The fail line and the heatmap colors are in the development rule.
