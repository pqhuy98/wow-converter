---
name: shot-export-wowhead
description: Screenshot the Wowhead model-viewer canvas for a wowhead.com NPC, item, or object page. Use when the user pastes a Wowhead URL, asks how a model looks on Wowhead, or wants that canvas compared with the wow-converter export.
---

# Shot export (Wowhead)

Save only the model-viewer canvas. The page, toolbar, and cookie banner are not in the PNG. The shark pattern is removed. The background is the same flat gray as the converter viewer.

Append `#modelviewer` when the URL does not already have it.

From the wow-converter repo:

```
bun .cursor/skills/shot-export-wowhead/snapshot.ts <url>
```

Optional flags: `--seq Attack` (default Stand), `--view front` (default is all six: front, back, left, right, top, bottom), `--out dir`. A second positional argument is also an output directory.

`<seq>` is the Wowhead animation name, such as `Stand`, `Attack1H`, or `Death`. One unique prefix matches. `Attack` picks `AttackUnarmed`, then `Attack1H`, then `Attack2H` when several names start with that prefix. The pose is the middle of that animation.

WoW names and WC3 sequence names are different. Look up the pair in `src/lib/converter/wow-model/animation/animation-mapper.ts` (`getWc3AnimName`: the case label is the Wowhead name, `wc3Name` is the converter name). Pass the Wowhead name to this script and `wc3Name` to shot-export-wow-converter. `Stand 1` and `Attack` are WC3 names, so they do not belong in this `--seq`.

Views match the converter shot, with the model facing the viewer on `front`: back is from behind, left looks from the model's left toward its right, right is the opposite, top looks down, bottom looks up.

The script prints one PNG path per view, named `<slug>-<seq>-<view>.png`. Read those images. A blank frame, or a mesh with holes where the face or cloth should be, means the shot failed.

When the user pasted the URL to see the difference, also take the wow-converter shot (shot-export-wow-converter) with the same `--seq` and `--view` and read both PNGs.
