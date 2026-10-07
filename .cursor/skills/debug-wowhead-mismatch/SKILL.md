---
name: debug-wowhead-mismatch
description: Investigate and fix a visual mismatch between a wowhead.com model and the wow-converter export. Use when the user pastes a Wowhead NPC, item, or object URL and says the converter looks different, wrong, or missing a part.
---

# Debug a Wowhead mismatch

The shot skills only capture PNGs. This skill decides why those PNGs differ and where to change the converter.

`bun run dev` is the live Go server on http://127.0.0.1:3001. Fix the owning Go layer and trace its character/map callers. Update a TypeScript path when the supported compatibility workflow is affected; do not duplicate Go-only work for parity (ADR 0001 in `docs/decisions/`).

Investigation logs, scripts, and artifacts go in the repo `tmp/` folder. `tmp/` is gitignored.

## Shots

Prefer one paired command so both sources share cameras:

```
go -C golang run ./cmd/shot-converter <asset-path> --seq "Stand 1" --wowhead <url> --wow-seq Stand --view front --out tmp/shots/<slug>
```

Standalone: `go -C golang run ./cmd/shot-wowhead` and `go -C golang run ./cmd/shot-converter` (`bun run shot` is the converter CLI). Pass `--out tmp/shots/<slug>`. Do not write PNGs under `.cursor/skills/`. Default out is repo `tmp/shots`.

Shoot one view that shows the difference (`front` unless the difference is elsewhere). Wowhead `--seq` is the WoW name; converter `--seq` is WC3 `name` from `result.reportMetadata.models[].sequences[]` (`wowName` / `wowVariant` vs `name`). Runtime table: `golang/internal/converter/wowmodel/animation/` (generated from `src/lib/converter/wow-model/animation/animation-mapper.ts`). Read both PNGs before editing.

A blank Wowhead frame, or holes where a face or cloth should be, means that shot failed. Re-shoot it. The same look from wow-converter can be a conversion bug.

Name the difference in one line, then open only that layer:

- a part is absent
- the part is there but the wrong color, black, or magenta
- the pose or timing differs

## Absent part

Export `format: "mdl"` and `formatVersion: "1000"`. Read geoset `Name` lines. Do not hex-edit MDX, and do not download the M2 to parse submeshes. Port 3001 serves `/api`; other paths are the UI.

`Name` comes from `GetGeosetName` in `golang/internal/wow/export/m2/geoset_mapper.go`. Submesh id 0 is `Geoset` plus the submesh index. Any other id is the group label plus `id % 100`, so id 2 is `Hair2` and id 702 is `Ears2`. A part that is not in the MDL was not exported. Vertex bounds confirm it: the missing part's extent is absent.

The mask is `BuildGeosetMaskForSkin` in `golang/internal/converter/wowmodel/direct/m2/convert.go`. With no extra geosets it keeps ids ending in `0` or `01`, every id under 100, and a 100-group that has a single distinct id which is not a suffix default. Two non-default variants in one group stay off. When the display lists extra geosets, ids from 1 through 899 start off and only the listed ids turn on. Id 0 and ids at or above 900 stay on.

Wowhead `meta/npc/<displayId>.json` can contain a `Creature` object whose `CreatureGeosetData` is null. That is an empty list. `mergeNpcMeta` keeps Wowhead's creature when the object is present, so the empty list does not fall through to the DB geosets.

`Chosen skin` in the export log is the texture file's base name, not a `.skin` file. The score is texture hits when no geoset contributed. A geoset contribution is multiplied by 1000000, so a small score means geosets did not choose the skin. Confidence below 100% means at least one wanted texture id was not on that skin.

The MDL only lists geosets the mask kept. To see a dropped id, log the mask for that one export, read it, then delete the log.

## Wrong color, black, or magenta

Read `Textures` and `Materials` in the MDL: which bitmap each geoset uses, `FilterMode`, and `Unlit`. A present mesh with a bad surface is a texture or material bug. A missing mesh is the geoset mask.

Creature color variants are the skin pick above. Dressing-room dye is the item-bonus display modifier (`displayModByBonus` in `golang/internal/wowhead/gatherer_items.go`), not this skin pick. WMO black or magenta: shader 23 binds diffuse from texture2 (skip a filled texture1) in `wmoDiffuseFileID`; missing shader-23 environment textures must not emit an empty pass (ADR 0002). M2 native opaque/alpha coverage is `golang/internal/converter/wowmodel/direct/m2/texture_native_opaque.go`.

## Wrong pose

Confirm the two shots used the mapped animation pair, frozen at mid-sequence. `Stand` on the converter matches `Stand 1`. A difference caused by comparing those names as if they were different animations is not a bug. A difference at the same mapped sequence is mapping or timing.

## After a fix

Re-export, shoot the same `--out` and `--view`, and read both PNGs again. A geoset-mask change applies to every creature that uses the default mask. Re-shoot the reported model. Run `bun test tests/snapshot-tests/model/model.test.ts` before treating other catalog sheets as still valid. The converter must already be listening.

Once the user confirms the shots match, add a folder `tests/snapshot-tests/model/<suite>/<slug>/` with `<slug>.manifest.json` (`retail`, `classic`, or `mount`; set `case.textureBaking` for bake cases) and write its expected sheet. `bun test` does not forward arguments after `--`. Use:

```
SNAPSHOT_UPDATE=1 SNAPSHOT_SUITE=retail SNAPSHOT_SLUG=<slug> bun test tests/snapshot-tests/model/model.test.ts
```

Read the new expected image. Do not rewrite an existing sheet unless the user explicitly asks.

Delete temporary logs from the converter source. Leave the shot CLIs unchanged.
